package apix

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/WnJee/gorig/utils/errors"
	"github.com/go-playground/validator/v10"
)

var (
	bindingValidator  *validator.Validate
	validateValidator *validator.Validate
	validateOnce      sync.Once
)

func registerTimeValidators(v *validator.Validate) {
	// 1. datetime validator: default "2006-01-02 15:04:05" or RFC3339, or custom layout via param
	_ = v.RegisterValidation("datetime", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()
		if val == "" {
			return true
		}
		param := fl.Param()
		if param != "" {
			_, err := time.Parse(param, val)
			return err == nil
		}
		for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02T15:04:05"} {
			if _, err := time.Parse(layout, val); err == nil {
				return true
			}
		}
		return false
	})

	// 2. date validator: default "2006-01-02" or custom layout via param
	_ = v.RegisterValidation("date", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()
		if val == "" {
			return true
		}
		param := fl.Param()
		if param != "" {
			_, err := time.Parse(param, val)
			return err == nil
		}
		for _, layout := range []string{"2006-01-02", "2006/01/02", "20060102"} {
			if _, err := time.Parse(layout, val); err == nil {
				return true
			}
		}
		return false
	})

	// 3. time validator: default "15:04:05" or "15:04", or custom layout via param
	_ = v.RegisterValidation("time", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()
		if val == "" {
			return true
		}
		param := fl.Param()
		if param != "" {
			_, err := time.Parse(param, val)
			return err == nil
		}
		for _, layout := range []string{"15:04:05", "15:04"} {
			if _, err := time.Parse(layout, val); err == nil {
				return true
			}
		}
		return false
	})
}

func initValidators() {
	validateOnce.Do(func() {
		tagNameFunc := func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			if name == "" {
				name = strings.SplitN(fld.Tag.Get("form"), ",", 2)[0]
			}
			if name == "" {
				name = fld.Name
			}
			return name
		}

		// Gin convention: binding:"..."
		bindingValidator = validator.New()
		bindingValidator.SetTagName("binding")
		bindingValidator.RegisterTagNameFunc(tagNameFunc)
		registerTimeValidators(bindingValidator)

		// Standard Go validator convention: validate:"..."
		validateValidator = validator.New()
		validateValidator.RegisterTagNameFunc(tagNameFunc)
		registerTimeValidators(validateValidator)
	})
}

// GetValidator returns the global validator instance (defaults to binding tag)
func GetValidator() *validator.Validate {
	initValidators()
	return bindingValidator
}

// ValidateStruct validates a struct using validation tags (supports both binding:"..." and validate:"...")
func ValidateStruct(obj interface{}) *errors.Error {
	if obj == nil {
		return nil
	}
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil
	}

	initValidators()

	// 1. Validate against Gin's binding tag
	if err := bindingValidator.Struct(obj); err != nil {
		if formatted := formatValidationError(err); formatted != nil {
			return formatted
		}
	}

	// 2. Validate against standard validate tag
	if err := validateValidator.Struct(obj); err != nil {
		if formatted := formatValidationError(err); formatted != nil {
			return formatted
		}
	}

	return nil
}

func formatValidationError(err error) *errors.Error {
	if validationErrs, ok := err.(validator.ValidationErrors); ok {
		var errMsgs []string
		for _, e := range validationErrs {
			field := e.Field()
			tag := e.Tag()
			param := e.Param()
			switch tag {
			case "required":
				errMsgs = append(errMsgs, fmt.Sprintf("%s is required", field))
			case "min":
				errMsgs = append(errMsgs, fmt.Sprintf("%s must be at least %s", field, param))
			case "max":
				errMsgs = append(errMsgs, fmt.Sprintf("%s must be at most %s", field, param))
			case "email":
				errMsgs = append(errMsgs, fmt.Sprintf("%s must be a valid email address", field))
			case "len":
				errMsgs = append(errMsgs, fmt.Sprintf("%s length must be %s", field, param))
			case "oneof":
				errMsgs = append(errMsgs, fmt.Sprintf("%s must be one of [%s]", field, param))
			case "datetime":
				if param != "" {
					errMsgs = append(errMsgs, fmt.Sprintf("%s must be formatted as '%s'", field, param))
				} else {
					errMsgs = append(errMsgs, fmt.Sprintf("%s must be a valid datetime (e.g. '2006-01-02 15:04:05')", field))
				}
			case "date":
				if param != "" {
					errMsgs = append(errMsgs, fmt.Sprintf("%s must be formatted as '%s'", field, param))
				} else {
					errMsgs = append(errMsgs, fmt.Sprintf("%s must be a valid date (e.g. '2006-01-02')", field))
				}
			case "time":
				if param != "" {
					errMsgs = append(errMsgs, fmt.Sprintf("%s must be formatted as '%s'", field, param))
				} else {
					errMsgs = append(errMsgs, fmt.Sprintf("%s must be a valid time (e.g. '15:04:05')", field))
				}
			default:
				if param != "" {
					errMsgs = append(errMsgs, fmt.Sprintf("%s failed validation on '%s(%s)'", field, tag, param))
				} else {
					errMsgs = append(errMsgs, fmt.Sprintf("%s failed validation on '%s'", field, tag))
				}
			}
		}
		return errors.Verify(strings.Join(errMsgs, "; "))
	}

	return errors.Verify(fmt.Sprintf("validation failed: %v", err))
}
