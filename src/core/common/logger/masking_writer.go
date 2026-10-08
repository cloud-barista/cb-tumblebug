package logger

import (
	"bytes"
	"io"
	"regexp"

	"github.com/rs/zerolog"
)

// sensitiveKeywords are used for fast-path check before running regular expressions.
var sensitiveKeywords = [][]byte{
	[]byte("password"),
	[]byte("passwd"),
	[]byte("secret"),
	[]byte("token"),
	[]byte("privatekey"),
	[]byte("private_key"),
	[]byte("credential"),
	[]byte("auth"),
	[]byte("-----begin"),
	[]byte("certificate"),
	[]byte("cacertificate"),
	[]byte("pem"),
	[]byte("bearer "),
}

// pemBlockPattern matches PEM-encoded blocks (certificates, private keys, etc.).
var pemBlockPattern = regexp.MustCompile(
	`(?s)-----BEGIN ([A-Z0-9_\- ]+)-----.*?-----END [A-Z0-9_\- ]+-----(?:\r?\n|\\n)?`,
)

// bearerTokenPattern matches HTTP Bearer authentication tokens.
var bearerTokenPattern = regexp.MustCompile(
	`(?i)(bearer\s+)[A-Za-z0-9\-\._~\+\/]+=*`,
)

// escapedJsonSensitivePattern matches escaped JSON key-value pairs within string fields: \"key\":\"val\"
var escapedJsonSensitivePattern = regexp.MustCompile(
	`(?i)(\\"[^"\\]*(?:password|passwd|secret|token|privatekey|private_key|credential|auth)[^"\\]*\\")\s*:\s*(\\"(?:[^\\"]|\\.)*\\")`,
)

// jsonSensitivePattern matches standard JSON key-value pairs with sensitive keys.
var jsonSensitivePattern = regexp.MustCompile(
	`(?i)(^|[^\\])("(?:[^"\\]*(?:password|passwd|secret|token|privatekey|private_key|credential|auth)[^"\\]*)")\s*:\s*("(?:[^"\\]|\\.)*"|true|false|null|-?[0-9]+(?:\.[0-9]+)?)`,
)

// textSensitivePattern matches config-style bracketed parameters like <TB_POSTGRES_PASSWORD> value.
var textSensitivePattern = regexp.MustCompile(
	`(?i)(<[A-Z0-9_]*(?:PASSWORD|PASSWD|SECRET|TOKEN|KEY|CERT|PEM)[A-Z0-9_]*>\s*)([^\s"'\\}\]\)]+)`,
)

// kvSensitivePattern matches URL query parameters, Go struct fields, or key-value logs like password=value.
var kvSensitivePattern = regexp.MustCompile(
	`(?i)((?:password|passwd|secret|token|private_?key|credential|auth)[=:]\s*)([^\s,;&"'\\}\]\)]+)`,
)

// MaskingWriter intercepts the log output stream and masks sensitive data.
type MaskingWriter struct {
	w io.Writer
}

// NewMaskingWriter wraps an io.Writer with sensitive data masking.
func NewMaskingWriter(w io.Writer) *MaskingWriter {
	return &MaskingWriter{w: w}
}

// Write intercepts the log bytes, masks any sensitive information, and forwards to the underlying writer.
// Returns len(p) on success to satisfy the io.Writer contract regardless of masked byte length differences.
func (mw *MaskingWriter) Write(p []byte) (n int, err error) {
	masked := MaskSensitiveData(p)
	if _, err := mw.w.Write(masked); err != nil {
		return 0, err
	}
	return len(p), nil
}

// WriteLevel implements zerolog.LevelWriter for level-aware writers.
func (mw *MaskingWriter) WriteLevel(level zerolog.Level, p []byte) (n int, err error) {
	masked := MaskSensitiveData(p)
	if lw, ok := mw.w.(zerolog.LevelWriter); ok {
		if _, err := lw.WriteLevel(level, masked); err != nil {
			return 0, err
		}
	} else {
		if _, err := mw.w.Write(masked); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

// MaskSensitiveData scans and replaces sensitive fields with fixed-length asterisks or masked tags.
func MaskSensitiveData(data []byte) []byte {
	lower := bytes.ToLower(data)
	matched := false
	for _, kw := range sensitiveKeywords {
		if bytes.Contains(lower, kw) {
			matched = true
			break
		}
	}
	if !matched {
		return data
	}

	res := pemBlockPattern.ReplaceAll(data, []byte(`[MASKED $1]`))
	res = bearerTokenPattern.ReplaceAll(res, []byte(`${1}********`))
	res = escapedJsonSensitivePattern.ReplaceAll(res, []byte(`$1:\"********\"`))
	res = jsonSensitivePattern.ReplaceAll(res, []byte(`${1}${2}:"********"`))
	res = textSensitivePattern.ReplaceAll(res, []byte(`${1}********`))
	res = kvSensitivePattern.ReplaceAll(res, []byte(`${1}********`))
	return res
}
