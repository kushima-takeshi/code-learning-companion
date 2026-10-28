package learningrecord

import "encoding/json"

func DecodeSaveInput(data []byte) (SaveInput, error) {
	var input SaveInput

	if err := json.Unmarshal(data, &input); err != nil {
		return SaveInput{}, err
	}

	if err := input.Validate(); err != nil {
		return SaveInput{}, err
	}

	return input, nil
}
