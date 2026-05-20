package validation

import (
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/pt"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	pt_translations "github.com/go-playground/validator/v10/translations/pt"
)

var translator ut.Translator

func init() {
	if validator, ok := binding.Validator.Engine().(*validator.Validate); ok {
		ptLocale := pt.New()

		uni := ut.New(ptLocale, ptLocale)

		translator, _ = uni.GetTranslator("pt")

		pt_translations.RegisterDefaultTranslations(validator, translator)

	}
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message,omitempty"`
}

func ToFieldError(validationErrs validator.ValidationErrors) []FieldError {
	causes := make([]FieldError, 0, len(validationErrs))

	for _, e := range validationErrs {
		causes = append(causes, FieldError{
			Field:   e.Field(),
			Message: e.Translate(translator),
		})
	}

	return causes
}
