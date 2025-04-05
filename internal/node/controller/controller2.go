package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bluesky-social/indigo/api/agnostic"
	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/data"
	"github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/controller/encrypter"
	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/node/reverse_proxy"
	"github.com/eagraf/habitat-new/internal/package_manager"
	"github.com/eagraf/habitat-new/internal/process"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type Controller2 struct {
	ctx            context.Context
	db             hdb.Client
	processManager process.ProcessManager
	pkgManagers    map[node.DriverType]package_manager.PackageManager
	proxyServer    *reverse_proxy.ProxyServer

	// use for encrypted record wrappers on top of pds
	e encrypter.Encrypter
}

func NewController2(
	ctx context.Context,
	processManager process.ProcessManager,
	pkgManagers map[node.DriverType]package_manager.PackageManager,
	db hdb.Client,
	proxyServer *reverse_proxy.ProxyServer,
	xrpcClient *xrpc.Client,
	encrypter encrypter.Encrypter,
) (*Controller2, error) {
	// Validate types of all input components
	_, ok := processManager.(node.Component[process.RestoreInfo])
	if !ok {
		return nil, fmt.Errorf("Process manager of type %T does not implement Component[*node.Process]", processManager)
	}

	ctrl := &Controller2{
		ctx:            ctx,
		processManager: processManager,
		pkgManagers:    pkgManagers,
		db:             db,
		proxyServer:    proxyServer,
		e:              encrypter,
	}

	return ctrl, nil
}

func (c *Controller2) getNodeState() (*node.State, error) {
	var nodeState node.State
	err := json.Unmarshal(c.db.Bytes(), &nodeState)
	if err != nil {
		return nil, err
	}
	return &nodeState, nil
}

func (c *Controller2) startProcess(installationID string) error {
	state, err := c.getNodeState()
	if err != nil {
		return fmt.Errorf("error getting node state: %s", err.Error())
	}

	app, ok := state.AppInstallations[installationID]
	if !ok {
		return fmt.Errorf("app with ID %s not found", installationID)
	}

	transition, err := node.GenProcessStartTransition(installationID, state)
	if err != nil {
		return errors.Wrap(err, "error creating transition")
	}

	newJSONState, err := c.db.ProposeTransitions([]hdb.Transition{transition})
	if err != nil {
		return errors.Wrap(err, "error proposing transition")
	}

	var newState node.State
	err = newJSONState.Unmarshal(&newState)
	if err != nil {
		return errors.Wrap(err, "error getting new state")
	}

	err = c.processManager.StartProcess(c.ctx, transition.Process.ID, app)
	if err != nil {
		// Rollback the state change if the process start failed
		_, err = c.db.ProposeTransitions([]hdb.Transition{
			&node.ProcessStopTransition{
				ProcessID: transition.Process.ID,
			},
		})
		return errors.Wrap(err, "error starting process")
	}

	// Register with reverse proxy server
	for _, rule := range newState.ReverseProxyRules {
		if rule.AppID == transition.Process.AppID {
			if c.proxyServer.RuleSet.AddRule(rule) != nil {
				return errors.Wrap(err, "error adding reverse proxy rule")
			}
		}
	}

	return nil
}

func (c *Controller2) stopProcess(processID node.ProcessID) error {
	procErr := c.processManager.StopProcess(c.ctx, processID)
	// If there was no process found with this ID, continue with the state transition
	// Otherwise this action failed, return an error without the transition
	if procErr != nil && !errors.Is(procErr, process.ErrNoProcFound) {
		// process.ErrNoProcFound is sometimes expected. In this case, still
		// attempt to remove the process from the node state.
		return procErr
	}

	// Only propose transitions if the process exists in state
	_, err := c.db.ProposeTransitions([]hdb.Transition{
		&node.ProcessStopTransition{
			ProcessID: processID,
		},
	})
	return err
}

func (c *Controller2) installApp(userID string, pkg *node.Package, version string, name string, proxyRules []*node.ReverseProxyRule, start bool) error {
	installer, ok := c.pkgManagers[pkg.Driver]
	if !ok {
		return fmt.Errorf("No driver %s found for app installation [name: %s, version: %s, package: %v]", pkg.Driver, name, version, pkg)
	}

	transition := node.GenStartInstallationTransition(userID, pkg, version, name, proxyRules)
	_, err := c.db.ProposeTransitions([]hdb.Transition{
		transition,
	})
	if err != nil {
		return err
	}

	err = installer.InstallPackage(pkg, version)
	if err != nil {
		return err
	}
	_, err = c.db.ProposeTransitions([]hdb.Transition{
		&node.FinishInstallationTransition{
			AppID: transition.ID,
		},
	})
	if err != nil {
		return err
	}

	if start {
		return c.startProcess(transition.ID)
	}
	return nil
}

func (c *Controller2) uninstallApp(appID string) error {
	_, err := c.db.ProposeTransitions([]hdb.Transition{
		&node.UninstallTransition{
			AppID: appID,
		},
	})

	return err
}

func (c *Controller2) restore(state *node.State) error {
	// Restore app installations to desired state
	for _, pkgManager := range c.pkgManagers {
		err := pkgManager.RestoreFromState(c.ctx, state.AppInstallations)
		if err != nil {
			return err
		}
	}

	// Restore reverse proxy rules to the desired state
	for _, rule := range state.ReverseProxyRules {
		err := c.proxyServer.RuleSet.AddRule(rule)
		if err != nil {
			log.Error().Msgf("error restoring rule: %s", err)
		}
	}

	// Restore processes to the current state
	info := make(map[node.ProcessID]*node.AppInstallation)
	for _, proc := range state.Processes {
		app, ok := state.AppInstallations[proc.AppID]
		if !ok {
			return fmt.Errorf("no app installation found for desired process: ID=%s appID=%s", proc.ID, proc.AppID)
		}
		info[proc.ID] = app
	}

	return c.processManager.RestoreFromState(c.ctx, info)
}

const encryptedRecordNSID = "com.habitat.encryptedRecord"

func encryptedRecordRKey(collection string, rkey string) string {
	return fmt.Sprintf("enc:%s:%s", collection, rkey)
}

type encryptedRecord struct {
	Data util.BlobSchema `json:"data" cborgen:"data"`
}

var (
	ErrPublicRecordExists               = fmt.Errorf("a public record exists with the same key")
	ErrNoPutsOnEncryptedRecord          = fmt.Errorf("directly put-ting to this lexicon is not valid")
	ErrNoEncryptedGetsOnEncryptedRecord = fmt.Errorf("calling getEncryptedRecord on a com.habitat.encryptedRecord is not supported")
)

// Returns true if err indicates the RecordNotFound error
func errorIsNoRecordFound(err error) bool {
	// TODO: Not sure if the atproto lib has an error to directly use with errors.Is()
	return strings.Contains(err.Error(), "RecordNotFound") || strings.Contains(err.Error(), "Could not locate record")
}

// type encryptedRecord map[string]any
// the shape of the lexicon is { "cid": <cid pointing to the encrypted blob> }

// putRecord with encryption wrapper around this
// ONLY YOU CAN CALL PUT RECORD, NO ONE ELSE
// Our security relies on this -- if this wasn't true then theoretically an attacker could call putRecord trying different rkey.
// If they were unable to create with an rkey, that means that it exists privately.
func (c *Controller2) putRecord(ctx context.Context, xrpc *xrpc.Client, input *agnostic.RepoPutRecord_Input, encrypt bool) (*agnostic.RepoPutRecord_Output, error) {
	if input.Collection == encryptedRecordNSID {
		return nil, ErrNoPutsOnEncryptedRecord
	}

	// Not encrypted -- blindly forward the request to PDS
	if !encrypt {
		return agnostic.RepoPutRecord(ctx, xrpc, input)
	}

	// Check if a record under this collection already exists publicly with this rkey
	// if so, return error (need a different rkey)
	_, err := agnostic.RepoGetRecord(ctx, xrpc, "", input.Collection, input.Repo, input.Rkey)
	if err == nil {
		return nil, fmt.Errorf("%w: %s", ErrPublicRecordExists, input.Rkey)
	} else if !errorIsNoRecordFound(err) {
		return nil, err
	}

	// If we're here, then this record is to be encrypted and there is no existing public record with rkey
	// Encrypted -- unpack the request and use special habitat encrypted record lexicon
	return c.putEncryptedRecord(ctx, xrpc, input.Collection, input.Repo, input.Record, input.Rkey, input.Validate)
}

type GetRecordResponse struct {
	Cid   *string `json:"cid"`
	Uri   string  `json:"uri"`
	Value any     `json:"value"`
}

// There are some different scenarios here: (TODO: write tests for all of these scenarios)
//
//	1a) cid = that of a non-com.habitat.encryptedRecord --> return that data as-is.
//	1b) cid = that of a com.habitat.encryptedRecord --> return that data as-is, it will simply be encrypted. getRecord will not attempt to authz and decrypt.
//	1c) cid = that of a private or public blob --> return that blob as-is.
//
// If no cid is provided, fallback to using collection + rkey as the lookup:
//
//	2a) collection + rkey = a com.habitat.encryptedRecord --> return that data as-is if exists, which contains a cid pointer to a blob. if no such record exists, return
//	--) collection + rkey = a non-com.habitat.encryptedRecord:
//	   2b) if a corresponding record is found, return that
//	   2c) if no corresponding record is found, attempt to decrypt the record a com.habitat.encryptedRecord would point to for that collection + rkey
func (c *Controller2) getRecord(ctx context.Context, xrpc *xrpc.Client, cid string, collection string, did string, rkey string) (*agnostic.RepoGetRecord_Output, error) {
	// Attempt to get a public record corresponding to the Collection + Repo + Rkey.
	// If the given cid does not point to anything, the GetRecord endpoint returns an error.
	// Record not found results in an error, as does any other non-200 response from the endpoint.
	// Cases 1a - 1c are handled directly by this case.
	output, err := agnostic.RepoGetRecord(ctx, xrpc, cid, collection, did, rkey)
	// If this is a cid lookup (cases 1a-1c) or the record was found (2a + 2b), simply return ()
	if err == nil {
		return output, nil
	}

	// If the record with the given collection + rkey identifier was not found (case 2c), attempt to get a private record with permissions look up.
	if strings.Contains(err.Error(), "RecordNotFound") || strings.Contains(err.Error(), "Could not locate record") {
		return c.getEncryptedRecord(ctx, xrpc, cid, collection, did, rkey)
	}
	// Otherwise the lookup failed in some other way, return the error
	return nil, err
}

// getEncryptedRecord assumes that the record given by the cid + collection + rkey + did has been encrypted via putRecord and fetches it
func (c *Controller2) getEncryptedRecord(ctx context.Context, xrpc *xrpc.Client, cid string, collection string, did string, rkey string) (*agnostic.RepoGetRecord_Output, error) {
	if collection == encryptedRecordNSID {
		return nil, ErrNoEncryptedGetsOnEncryptedRecord
	}
	encKey := encryptedRecordRKey(collection, rkey)
	output, err := agnostic.RepoGetRecord(ctx, xrpc, cid, encryptedRecordNSID, did, encKey)
	if err != nil {
		return nil, err
	}

	// Run permissions before returning to the user
	// if HasAccess(did, collection, rkey) { .... }
	var record encryptedRecord
	// Unfortunate that we need to MarshalJSON to turn it back into bytes -- the RepoGetRecord function probably Unmarshals :/
	bytes, err := output.Value.MarshalJSON()
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytes, &record)
	if err != nil {
		return nil, err
	}

	// blob contains the encrypted lexicon written by the user
	blob, err := atproto.SyncGetBlob(ctx, xrpc, record.Data.Ref.String(), did)
	if err != nil {
		return nil, err
	}

	dec, err := c.e.Decrypt(rkey, blob)
	if err != nil {
		return nil, err
	}

	asJson := json.RawMessage(dec)
	return &agnostic.RepoGetRecord_Output{
		Cid:   output.Cid,
		Uri:   output.Uri,
		Value: &asJson,
	}, nil
}

// puttEncryptedRecord encrypts the given record.
func (c *Controller2) putEncryptedRecord(ctx context.Context, xrpc *xrpc.Client, collection string, repo string, record map[string]any, rkey string, validate *bool) (*agnostic.RepoPutRecord_Output, error) {
	if collection == encryptedRecordNSID {
		return nil, ErrNoPutsOnEncryptedRecord
	}
	// If we're here, then this record is to be encrypted and there is no existing public record with rkey
	// Encrypted -- unpack the request and use special habitat encrypted record lexicon
	marshalled, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}

	enc, err := c.e.Encrypt(rkey, marshalled)
	if err != nil {
		return nil, err
	}

	if validate != nil && *validate {
		err = data.Validate(record)
		if err != nil {
			return nil, err
		}
	}

	blobOut, err := atproto.RepoUploadBlob(ctx, xrpc, bytes.NewBuffer(enc))
	if err != nil {
		return nil, err
	}

	// CID is returned on uploadBlob
	blob := blobOut.Blob
	encKey := encryptedRecordRKey(collection, rkey)
	// It's our fault if this fails, but always attempt to validate the habitat encoded request
	validateEnc := false
	// TODO: let's make a helper function for this
	encInput := &agnostic.RepoPutRecord_Input{
		Collection: encryptedRecordNSID,
		Repo:       repo,
		Rkey:       encKey,
		Validate:   &validateEnc,
		Record: map[string]any{
			"data": blob,
		},
	}
	return agnostic.RepoPutRecord(ctx, xrpc, encInput)
}
