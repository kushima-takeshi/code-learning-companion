package learningrecord

import (
	"bytes"
	"encoding/json"
)

// MaxSaveInputBytes is the maximum size of the complete incoming JSON, in bytes.
const MaxSaveInputBytes = 1 << 20

func DecodeSaveInput(data []byte) (SaveInput, error) {
	if len(data) > MaxSaveInputBytes {
		return SaveInput{}, ErrSaveInputTooLarge
	}

	var input SaveInput

	// Keep the single-JSON contract and the existing json.SyntaxError behavior.
	if !json.Valid(data) {
		return SaveInput{}, json.Unmarshal(data, &input)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return SaveInput{}, err
	}

	if err := input.Validate(); err != nil {
		return SaveInput{}, err
	}

	return input, nil
}
