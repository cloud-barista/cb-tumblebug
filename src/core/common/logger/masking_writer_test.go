package logger

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestMaskSensitiveData(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "RDBMS request body with MasterUserPassword",
			input:    `{"Method":"DELETE","URI":"http://cb-spider:1024/spider/rdbms","requestBody":{"ConnectionName":"nhn-kr1","MasterUserName":"myadmin","MasterUserPassword":"Password123!"}}`,
			expected: `{"Method":"DELETE","URI":"http://cb-spider:1024/spider/rdbms","requestBody":{"ConnectionName":"nhn-kr1","MasterUserName":"myadmin","MasterUserPassword":"********"}}`,
		},
		{
			name:     "Escaped JSON string with secret token",
			input:    `{"log":"{\"api_token\":\"secret-value-12345\"}"}`,
			expected: `{"log":"{\"api_token\":\"********\"}"}`,
		},
		{
			name:     "Config style DB password log",
			input:    `<TB_POSTGRES_PASSWORD> Password123!`,
			expected: `<TB_POSTGRES_PASSWORD> ********`,
		},
		{
			name:     "URL query param with password",
			input:    `Internal Call Method=GET URI=/api/test?password=secretPassword123&user=admin`,
			expected: `Internal Call Method=GET URI=/api/test?password=********&user=admin`,
		},
		{
			name:     "Key value with privateKey",
			input:    `resource created privateKey: my-rsa-private-key-data`,
			expected: `resource created privateKey: ********`,
		},
		{
			name:     "RDBMS CACertificate PEM in Go struct format",
			input:    `[Response from Spider] Getting RDBMS secure transport info: {Engine:mysql CACertificate:{PEM:-----BEGIN CERTIFICATE-----\nMIIDDzCCAfegAwIBAgIJANEH58y2/kzHMA0GCSqGSIb3DQEBCwUAMB4xHDAaBgNV\n-----END CERTIFICATE-----\n Subject:CN=IBM Cloud Databases} TLSInUse:true}`,
			expected: `[Response from Spider] Getting RDBMS secure transport info: {Engine:mysql CACertificate:{PEM:[MASKED CERTIFICATE] Subject:CN=IBM Cloud Databases} TLSInUse:true}`,
		},
		{
			name:     "Multiline PEM certificate block",
			input:    "server cert:\n-----BEGIN CERTIFICATE-----\nMIIDDzCCAfegAwIBAgI\n-----END CERTIFICATE-----\nstatus: active",
			expected: "server cert:\n[MASKED CERTIFICATE]status: active",
		},
		{
			name:     "Go struct with password at the end followed by closing delimiters",
			input:    `[Request to Spider] Creating RDBMS database (url: http://cb-spider:1024/spider/rdbms/tb094n6o6s88ckefvsb8/databases, request: {ConnectionName:ibm-us-south DatabaseName:sampledb MasterUserName:admin MasterUserPassword:Password123!})`,
			expected: `[Request to Spider] Creating RDBMS database (url: http://cb-spider:1024/spider/rdbms/tb094n6o6s88ckefvsb8/databases, request: {ConnectionName:ibm-us-south DatabaseName:sampledb MasterUserName:admin MasterUserPassword:********})`,
		},
		{
			name:     "Go struct already masked with password at the end followed by closing delimiters",
			input:    `[Request to Spider] Creating RDBMS database (url: http://cb-spider:1024/spider/rdbms/tb094n6o6s88ckefvsb8/databases, request: {ConnectionName:ibm-us-south DatabaseName:sampledb MasterUserName:admin MasterUserPassword:********})`,
			expected: `[Request to Spider] Creating RDBMS database (url: http://cb-spider:1024/spider/rdbms/tb094n6o6s88ckefvsb8/databases, request: {ConnectionName:ibm-us-south DatabaseName:sampledb MasterUserName:admin MasterUserPassword:********})`,
		},
		{
			name:     "VMUserPasswd in Spider JSON request body",
			input:    `{"Method":"POST","URI":"http://cb-spider:1024/spider/vm","requestBody":{"ConnectionName":"openstack-regionone","ReqInfo":{"VMUserId":"","VMUserPasswd":"ust1tqo!7rbA$s","RootDiskType":"default"}}}`,
			expected: `{"Method":"POST","URI":"http://cb-spider:1024/spider/vm","requestBody":{"ConnectionName":"openstack-regionone","ReqInfo":{"VMUserId":"","VMUserPasswd":"********","RootDiskType":"default"}}}`,
		},
		{
			name:     "HTTP Authorization Bearer token",
			input:    `curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.token" http://localhost:1323`,
			expected: `curl -H "Authorization: Bearer ********" http://localhost:1323`,
		},
		{
			name:     "Normal log with no sensitive keywords",
			input:    `New logger created with level=debug`,
			expected: `New logger created with level=debug`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(MaskSensitiveData([]byte(tt.input)))
			if got != tt.expected {
				t.Errorf("MaskSensitiveData() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestMaskingWriter_Write(t *testing.T) {
	buf := &bytes.Buffer{}
	mw := NewMaskingWriter(buf)

	input := []byte(`{"message":"user login","password":"secretPassword"}`)
	n, err := mw.Write(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(input) {
		t.Errorf("Write() returned n = %d, want len(p) = %d", n, len(input))
	}

	output := buf.String()
	if strings.Contains(output, "secretPassword") {
		t.Errorf("output contains unmasked password: %s", output)
	}
	if !strings.Contains(output, `"password":"********"`) {
		t.Errorf("output does not contain masked password: %s", output)
	}
}

func TestMaskingWriter_ZerologIntegration(t *testing.T) {
	buf := &bytes.Buffer{}
	mw := NewMaskingWriter(buf)
	logger := zerolog.New(mw).With().Logger()

	logger.Info().
		Str("user", "admin").
		Str("MasterUserPassword", "Password123!").
		Msg("Internal Call Start")

	output := buf.String()
	if strings.Contains(output, "Password123!") {
		t.Errorf("zerolog output leaked sensitive data: %s", output)
	}
	if !strings.Contains(output, `"MasterUserPassword":"********"`) {
		t.Errorf("zerolog output missing masked field: %s", output)
	}
}

func TestConfigureWriter_MaskingToggle(t *testing.T) {
	// 1. Masking enabled via configureWriter
	loggerEnabled := configureWriter("stdout", "json", zerolog.DebugLevel, true)
	if loggerEnabled == nil {
		t.Fatal("configureWriter returned nil")
	}

	// 2. Masking disabled via configureWriter
	loggerDisabled := configureWriter("stdout", "json", zerolog.DebugLevel, false)
	if loggerDisabled == nil {
		t.Fatal("configureWriter returned nil")
	}

	// 3. NewLogger with nil (default: enabled)
	lDefault := NewLogger(Config{LogWriter: "stdout"})
	if lDefault == nil {
		t.Fatal("NewLogger returned nil for default")
	}

	// 4. NewLogger with LogMaskingEnabled: true
	lTrue := NewLogger(Config{LogWriter: "stdout", LogMaskingEnabled: BoolPtr(true)})
	if lTrue == nil {
		t.Fatal("NewLogger returned nil for true")
	}

	// 5. NewLogger with LogMaskingEnabled: false
	lFalse := NewLogger(Config{LogWriter: "stdout", LogMaskingEnabled: BoolPtr(false)})
	if lFalse == nil {
		t.Fatal("NewLogger returned nil for false")
	}
}

func BenchmarkMaskSensitiveData_NormalLog(b *testing.B) {
	logMsg := []byte(`{"level":"info","time":"2026-10-07T18:00:00Z","caller":"src/core/resource/common.go:123","message":"fetching resource list for nsId=ns01"}`)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = MaskSensitiveData(logMsg)
	}
}

func BenchmarkMaskSensitiveData_SensitiveLog(b *testing.B) {
	logMsg := []byte(`{"level":"info","message":"call spider","body":{"ConnectionName":"nhn-kr1","MasterUserName":"myadmin","MasterUserPassword":"Password123!"}}`)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = MaskSensitiveData(logMsg)
	}
}

func BenchmarkZerolog_WithMasking(b *testing.B) {
	mw := NewMaskingWriter(io.Discard)
	logger := zerolog.New(mw)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info().Str("nsId", "ns01").Str("mcisId", "mcis01").Msg("processing request")
	}
}

func BenchmarkZerolog_WithoutMasking(b *testing.B) {
	logger := zerolog.New(io.Discard)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info().Str("nsId", "ns01").Str("mcisId", "mcis01").Msg("processing request")
	}
}
