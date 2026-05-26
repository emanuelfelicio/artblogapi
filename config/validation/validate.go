package validation

import (
	"regexp"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/pt"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	pt_translations "github.com/go-playground/validator/v10/translations/pt"
)

var translator ut.Translator

func init() {
	if engineValidator, ok := binding.Validator.Engine().(*validator.Validate); ok {
		ptLocale := pt.New()

		uni := ut.New(ptLocale, ptLocale)

		translator, _ = uni.GetTranslator("pt")

		pt_translations.RegisterDefaultTranslations(engineValidator, translator)
		_ = engineValidator.RegisterValidation("username_chars", validateUsernameChars)
		_ = engineValidator.RegisterValidation("maxbytes", validateMaxBytes)
		_ = engineValidator.RegisterTranslation("username_chars", translator, func(ut ut.Translator) error {
			return ut.Add("username_chars", "{0} deve conter apenas letras, números, '.' e '_'", true)
		}, func(ut ut.Translator, fe validator.FieldError) string {
			text, _ := ut.T("username_chars", fe.Field())
			return text
		})
		_ = engineValidator.RegisterTranslation("maxbytes", translator, func(ut ut.Translator) error {
			return ut.Add("maxbytes", "{0} deve ter no máximo {1} bytes", true)
		}, func(ut ut.Translator, fe validator.FieldError) string {
			text, _ := ut.T("maxbytes", fe.Field(), fe.Param())
			return text
		})

	}
}

var usernamePattern = regexp.MustCompile(`^[a-z0-9._]+$`)

func validateUsernameChars(fl validator.FieldLevel) bool {
	return usernamePattern.MatchString(fl.Field().String())
}

func validateMaxBytes(fl validator.FieldLevel) bool {
	return len(fl.Field().String()) <= 72
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
