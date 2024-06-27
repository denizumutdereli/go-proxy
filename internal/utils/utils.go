package utils

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"goproxy/internal/common"
	"goproxy/internal/types"

	"github.com/gin-gonic/gin"
)

func EnableCORS(next http.HandlerFunc, whitelist string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, Sec-WebSocket-Accept, Sec-WebSocket-Protocol, Sec-WebSocket-Key, Sec-WebSocket-Version, Sec-WebSocket-Extensions")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func CheckEnv(value string, envParam string) string {
	if value == "" {
		log.Fatalf("%s must be set", envParam)
	}
	return value
}

func GetClientIP(c *gin.Context) string {
	forwarded := c.GetHeader("X-Forwarded-For")
	var ip string
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		ip = strings.TrimSpace(ips[0])
	} else {
		ip = c.ClientIP()
	}
	return NormalizeIP(ip)
}

func NormalizeIP(ip string) string {

	// localhost
	if strings.Contains(ip, "::1") {
		return "127.0.0.1"
	}

	if strings.HasPrefix(ip, "::ffff:") {
		return strings.TrimPrefix(ip, "::ffff:")
	}
	return ip
}

func ClearScreen() {
	cmd := exec.Command("clear")
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		fmt.Print("\033[H\033[2J")
	}
}

func SplitString(s, delimeter string) []string {

	return strings.Split(s, delimeter)

}

func CleanInput(dsl string) []string {
	re := regexp.MustCompile(`['";@]`)
	sanitized := re.ReplaceAllString(dsl, "")
	conditions := strings.Split(sanitized, ",")
	return conditions
}

func ToCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i := range parts {
		parts[i] = strings.Title(parts[i])
	}
	return strings.Join(parts, "")
}

func GetKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func IsMn(r rune) bool {
	return unicode.Is(unicode.Mn, r) // Mn: nonspacing marks
}

func RemoveSpacesAndDots(s string) string {

	s = strings.ReplaceAll(s, " ", "")

	s = strings.ReplaceAll(s, ".", "")

	return s
}

func StringPrepareForComparision(s string) string {
	s = strings.ToUpper(s)
	s = strings.TrimSpace(s)
	return s
}

func IsServiceAllowed(name string, allowedServices []string) bool {
	for _, s := range allowedServices {
		if s == name {
			return true
		}
	}
	return false
}

func Contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}

func ParseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func FilterIncludedConfig(allowedKeys []string, fullConfig map[string]interface{}) map[string]interface{} {
	filteredConfig := make(map[string]interface{})

	for _, key := range allowedKeys {
		if value, ok := fullConfig[key]; ok {
			filteredConfig[key] = value
		}
	}

	return filteredConfig
}

func FilterExcludedConfig(excludedKeys []string, fullConfig map[string]interface{}) map[string]interface{} {
	filteredConfig := make(map[string]interface{})
	for k, v := range fullConfig {
		filteredConfig[k] = v
	}

	for _, key := range excludedKeys {
		delete(filteredConfig, key)
	}

	return filteredConfig
}

func PhoneValidator(phone string) bool {
	if !strings.HasPrefix(phone, "+") {
		return false
	}
	for _, char := range phone[1:] {
		if !unicode.IsDigit(char) {
			return false
		}
	}
	return len(phone) >= 12
}

func IfErrorExistReturnWithError(c *gin.Context, err common.Error) {
	if err != nil {
		errorResponse := types.ErrorResponse{Error: err.ErrorMessage(), Details: err.Error()}
		c.JSON(err.StatusCode(), errorResponse)
		return
	}
}

func IfErrorExistReturnWithErrorExplanation(c *gin.Context, err error, explanation string, statusCode ...int) {
	statusCodeInternal := http.StatusBadRequest
	if len(statusCode) > 0 {
		statusCodeInternal = statusCode[0]
	}

	var internalError common.Error

	if err != nil {
		internalError = common.AppError(statusCodeInternal, "", err.Error(), err)
	} else {
		internalError = common.AppError(statusCodeInternal, "", explanation, nil)
	}

	errorResponse := types.ErrorResponse{Error: internalError.ErrorMessage(), Details: internalError.Error()}
	c.JSON(statusCodeInternal, errorResponse)
}

func IfErrorExistReturnWithErrorDetails(c *gin.Context, err error, explanation string, details map[string]string, statusCode ...int) {
	statusCodeInternal := http.StatusBadRequest
	if len(statusCode) > 0 {
		statusCodeInternal = statusCode[0]
	}

	var internalError common.Error

	if err != nil {
		internalError = common.AppError(statusCodeInternal, "", err.Error(), err)
	} else {
		internalError = common.AppError(statusCodeInternal, "", explanation, nil)
	}

	errorResponse := types.ErrorResponse{Error: internalError.ErrorMessage(), Details: details}
	c.JSON(statusCodeInternal, errorResponse)
}

func NukeMe() {
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Start()
	if err != nil {
		log.Fatalf("Failed to restart: %s", err)
	}

	os.Exit(0)
}
