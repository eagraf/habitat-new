package fishtail

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/rs/zerolog/log"
)

// Traverse a CBOR object and call the processFunc for each visited node.
func traverseCBOR(recordCBOR []byte, processFunc func(node interface{}) error) error {

	var traverse func(v interface{})
	traverse = func(v interface{}) {
		log.Debug().Msgf("Visiting: %v", v)
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

	var obj interface{}
	if err := cbor.Unmarshal(recordCBOR, &obj); err != nil {
		return fmt.Errorf("failed to unmarshal CBOR: %s", err)
	}

	traverse(obj)

	return nil
}

// Traverse a JSON object and call the processFunc for each visited node.
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
