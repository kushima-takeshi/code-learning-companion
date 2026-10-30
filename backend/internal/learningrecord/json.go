package learningrecord

import "encoding/json"

// MaxSaveInputBytes is the maximum size of the complete incoming JSON, in bytes.
const MaxSaveInputBytes = 1 << 20

func DecodeSaveInput(data []byte) (SaveInput, error) {
	if len(data) > MaxSaveInputBytes {
		return SaveInput{}, ErrSaveInputTooLarge
	}

	var input SaveInput

	if err := json.Unmarshal(data, &input); err != nil {
		return SaveInput{}, err
	}

	if err := input.Validate(); err != nil {
		return SaveInput{}, err
	}

	return input, nil
}
