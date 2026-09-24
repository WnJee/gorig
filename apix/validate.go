package apix

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/WnJee/gorig/utils/errors"
	"github.com/go-playground/validator/v10"
)

var (
	validateInstance *validator.Validate
	validateOnce     sync.Once
)

// GetValidator returns the global validator instance
func GetValidator() *validator.Validate {
	validateOnce.Do(func() {
		validateInstance = validator.New()
		// Use json tag or form tag for field names in validation error messages
		validateInstance.RegisterTagNameFunc(func(fld reflect.StructField) string {
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
		})
	})
	return validateInstance
}

// ValidateStruct validates a struct using validation tags (validate:"..." and binding:"...")
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

	v := GetValidator()
	err := v.Struct(obj)
	if err == nil {
		return nil
	}

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
