package validator

// Validator is a type with helper functions for Validation
type Validator struct {
	Errors map[string]string
}

// Valid returns 'true' when there are no errors in the map
func (v Validator) Valid() bool {
	return !v.HasErrors()
}

// HasErrors returns 'true' when there are errors in the map
func (v Validator) HasErrors() bool {
	return len(v.Errors) != 0
}

// AddError adds a message for a given key to the map of errors.
func (v *Validator) AddError(key, message string) {
	if v.Errors == nil {
		v.Errors = map[string]string{}
	}

	if _, exists := v.Errors[key]; !exists {
		v.Errors[key] = message
	}
}

// Check will add an error message if the the 'ok' argument is false.
func (v *Validator) Check(ok bool, key, message string) {
	if !ok {
		v.AddError(key, message)
	}
}
