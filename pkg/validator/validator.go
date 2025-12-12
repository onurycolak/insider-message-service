package validator

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	"github.com/labstack/echo/v4"
)

// CustomValidator wraps the validator instance for Echo.
type CustomValidator struct {
	validator  *validator.Validate
	translator ut.Translator
}

func New() *CustomValidator {
	validate := validator.New()

	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		tag := field.Tag.Get("json")
		if tag == "" {
			return field.Name
		}

		name := strings.SplitN(tag, ",", 2)[0]
		if name == "-" || name == "" {
			return field.Name
		}

		return name
	})

	english := en.New()
	uni := ut.New(english, english)
	trans, _ := uni.GetTranslator("en")

	if err := enTranslations.RegisterDefaultTranslations(validate, trans); err != nil {
		panic("failed to register validator default translations: " + err.Error())
	}

	return &CustomValidator{
		validator:  validate,
		translator: trans,
	}
}

func (cv *CustomValidator) Validate(i any) error {
	if err := cv.validator.Struct(i); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			return &ValidationError{
				Errors: cv.translateErrors(validationErrors),
			}
		}
		return err
	}
	return nil
}

func (cv *CustomValidator) translateErrors(errs validator.ValidationErrors) map[string]string {
	errors := make(map[string]string)
	for _, err := range errs {
		field := err.Field()
		errors[field] = err.Translate(cv.translator)
	}
	return errors
}

type ValidationError struct {
	Errors map[string]string `json:"errors"`
}

func (e *ValidationError) Error() string {
	var messages []string
	for field, msg := range e.Errors {
		messages = append(messages, field+": "+msg)
	}
	return strings.Join(messages, "; ")
}

type ValidationErrorResponse struct {
	Success bool              `json:"success"`
	Error   string            `json:"error"`
	Details map[string]string `json:"details,omitempty"`
}

func HandleValidationError(c echo.Context, err error) error {
	if ve, ok := err.(*ValidationError); ok {
		return c.JSON(http.StatusUnprocessableEntity, ValidationErrorResponse{
			Success: false,
			Error:   "Validation failed",
			Details: ve.Errors,
		})
	}
	return c.JSON(http.StatusBadRequest, ValidationErrorResponse{
		Success: false,
		Error:   err.Error(),
	})
}
