package fishtail

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/rs/zerolog/log"
)

func traverseCBOR(recordCBOR []byte, processFunc func(node interface{}) error) error {
	log.Info().Msgf("Record CBOR bytes %s", fmt.Sprintf("%X", recordCBOR))

	// Pretty print the CBOR so it is human readable
	var prettyObj interface{}
	if err := cbor.Unmarshal(recordCBOR, &prettyObj); err != nil {
		return fmt.Errorf("failed to unmarshal CBOR for pretty printing: %s", err)
	}

	log.Info().Msgf("Pretty printed CBOR:\n%s", prettyObj)

	// Traverse the CBOR object and list all linked DIDs and rkeys
	var obj interface{}
	if err := cbor.Unmarshal(recordCBOR, &obj); err != nil {
		return fmt.Errorf("failed to unmarshal CBOR: %s", err)
	}

	var traverse func(v interface{})
	traverse = func(v interface{}) {
		err := processFunc(v)
		if err != nil {
			log.Error().Msgf("Error processing node: %s", err)
		}

		// Continue traversing the CBOR
		switch val := v.(type) {
		case map[interface{}]interface{}:
			for _, v := range val {
				traverse(v)
			}
		case []interface{}:
			for _, v := range val {
				traverse(v)
			}
		}
	}

	traverse(obj)

	return nil
}

func traverseJSON(recordJSON map[string]interface{}, processFunc func(node interface{}) error) error {

	var traverse func(v interface{}) error
	traverse = func(v interface{}) error {
		log.Debug().Msgf("Visiting: %v", v)
		if err := processFunc(v); err != nil {
			return err
		}

		switch val := v.(type) {
		case map[string]interface{}:
			for _, v := range val {
				if err := traverse(v); err != nil {
					return err
				}
			}
		case []interface{}:
			for _, v := range val {
				if err := traverse(v); err != nil {
					return err
				}
			}
		}

		return nil
	}

	return traverse(recordJSON)
}
