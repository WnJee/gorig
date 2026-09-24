package apix

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
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

	mobileRegexp = regexp.MustCompile(`^1[3-9]\d{9}$`)
	idCardRegexp = regexp.MustCompile(`^[1-9]\d{5}(18|19|20)\d{2}((0[1-9])|(1[0-2]))(([0-2][1-9])|10|20|30|31)\d{3}[0-9Xx]$`)
	semverRegexp = regexp.MustCompile(`^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)
)

func registerBuiltinValidators(v *validator.Validate) {
	// 1. datetime validator: default "2006-01-02 15:04:05", or custom layout via param
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
		_, err := time.Parse("2006-01-02 15:04:05", val)
		return err == nil
	})

	// 2. date validator: default "2006-01-02", or custom layout via param
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
		_, err := time.Parse("2006-01-02", val)
		return err == nil
	})

	// 3. time validator: default "15:04:05", or custom layout via param
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
		_, err := time.Parse("15:04:05", val)
		return err == nil
	})

	// 4. mobile validator
	_ = v.RegisterValidation("mobile", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()
		if val == "" {
			return true
		}
		return mobileRegexp.MatchString(val)
	})
	_ = v.RegisterValidation("phone", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()
		if val == "" {
			return true
		}
		return mobileRegexp.MatchString(val)
	})

	// 5. idcard validator
	_ = v.RegisterValidation("idcard", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()
		if val == "" {
			return true
		}
		return idCardRegexp.MatchString(val)
	})

	// 6. json_str validator
	_ = v.RegisterValidation("json_str", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()
		if val == "" {
			return true
		}
		var js json.RawMessage
		return json.Unmarshal([]byte(val), &js) == nil
	})

	// 7. semver validator
	_ = v.RegisterValidation("semver", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()
		if val == "" {
			return true
		}
		return semverRegexp.MatchString(val)
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
				name = strings.SplitN(fld.Tag.Get("query"), ",", 2)[0]
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
		registerBuiltinValidators(bindingValidator)

		// Standard Go validator convention: validate:"..."
		validateValidator = validator.New()
		validateValidator.RegisterTagNameFunc(tagNameFunc)
		registerBuiltinValidators(validateValidator)
	})
}

// GetValidator returns the global validator instance (defaults to binding tag).
func GetValidator() *validator.Validate {
	initValidators()
	return bindingValidator
}

// RegisterValidation registers a custom validation function to both binding and validate validators.
func RegisterValidation(tag string, fn validator.Func) error {
	initValidators()
	if err := bindingValidator.RegisterValidation(tag, fn); err != nil {
		return err
	}
	return validateValidator.RegisterValidation(tag, fn)
}

// ValidateStruct validates a struct using validation tags (supports both binding:"..." and validate:"...").
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

// ValidateVar validates a single variable against a validator tag rule (e.g. "required,email").
func ValidateVar(field any, tag string) *errors.Error {
	initValidators()
	if err := bindingValidator.Var(field, tag); err != nil {
		return formatValidationError(err)
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
			case "url":
				errMsgs = append(errMsgs, fmt.Sprintf("%s must be a valid URL", field))
			case "ip":
				errMsgs = append(errMsgs, fmt.Sprintf("%s must be a valid IP address", field))
			case "len":
				errMsgs = append(errMsgs, fmt.Sprintf("%s length must be %s", field, param))
			case "oneof":
				errMsgs = append(errMsgs, fmt.Sprintf("%s must be one of [%s]", field, param))
			case "mobile", "phone":
				errMsgs = append(errMsgs, fmt.Sprintf("%s must be a valid mobile phone number", field))
			case "idcard":
				errMsgs = append(errMsgs, fmt.Sprintf("%s must be a valid ID card number", field))
			case "json_str":
				errMsgs = append(errMsgs, fmt.Sprintf("%s must be a valid JSON string", field))
			case "semver":
				errMsgs = append(errMsgs, fmt.Sprintf("%s must be a valid semantic version", field))
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
