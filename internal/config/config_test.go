package config

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

type fakeKeychainStore struct {
	service string
	user    string
	token   string
	err     error
	sets    []fakeKeychainSet
	deletes []fakeKeychainDelete
}

type fakeKeychainSet struct {
	service  string
	user     string
	password string
}

type fakeKeychainDelete struct {
	service string
	user    string
}

func (f *fakeKeychainStore) Get(service, user string) (string, error) {
	f.service = service
	f.user = user
	if f.err != nil {
		return "", f.err
	}
	return f.token, nil
}

func (f *fakeKeychainStore) Set(service, user, password string) error {
	f.sets = append(f.sets, fakeKeychainSet{
		service:  service,
		user:     user,
		password: password,
	})
	return f.err
}

func (f *fakeKeychainStore) Delete(service, user string) error {
	f.deletes = append(f.deletes, fakeKeychainDelete{
		service: service,
		user:    user,
	})
	return f.err
}

func TestDefaultSourceForGOOS(t *testing.T) {
	tests := []struct {
		goos string
		want Source
	}{
		{goos: "darwin", want: SourceKeychain},
		{goos: "windows", want: SourceEnvOrFile},
		{goos: "linux", want: SourceEnvOrFile},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			if got := DefaultSourceForGOOS(tt.goos); got != tt.want {
				t.Fatalf("DefaultSourceForGOOS(%q) = %q, want %q", tt.goos, got, tt.want)
			}
		})
	}
}

func TestAuthConfigPath(t *testing.T) {
	resolver := NewResolver(Runtime{
		GOOS: "windows",
		UserConfigDir: func() (string, error) {
			return filepath.Join("C:", "Users", "alexis", "AppData", "Roaming"), nil
		},
		Getenv: func(string) string { return "" },
	}, nil)

	got, err := resolver.AuthConfigPath()
	if err != nil {
		t.Fatalf("AuthConfigPath() error = %v", err)
	}

	want := filepath.Join("C:", "Users", "alexis", "AppData", "Roaming", "zendesk-mgmt", "auth.json")
	if got != want {
		t.Fatalf("AuthConfigPath() = %q, want %q", got, want)
	}
}

func TestResolveTokenEnvWinsOverFile(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "zendesk-mgmt", "auth.json")
	if err := WriteFileConfig(path, FileConfig{Email: "file@example.com", APIToken: "file-token", AuthType: AuthTypeAPIToken}); err != nil {
		t.Fatalf("WriteFileConfig() error = %v", err)
	}

	resolver := NewResolver(Runtime{
		GOOS:          "windows",
		UserConfigDir: func() (string, error) { return tempDir, nil },
		Getenv: func(key string) string {
			if key == APITokenEnvVar {
				return "env-token"
			}
			if key == EmailEnvVar {
				return "env@example.com"
			}
			return ""
		},
	}, nil)

	got, err := resolver.ResolveToken(ResolveOptions{Source: SourceEnvOrFile})
	if err != nil {
		t.Fatalf("ResolveToken() error = %v", err)
	}
	if got.Token != "env-token" {
		t.Fatalf("ResolveToken() token = %q, want %q", got.Token, "env-token")
	}
	if got.Email != "env@example.com" {
		t.Fatalf("ResolveToken() email = %q, want %q", got.Email, "env@example.com")
	}
	if got.ResolvedFrom != "env" {
		t.Fatalf("ResolveToken() resolvedFrom = %q, want env", got.ResolvedFrom)
	}
}

func TestResolveTokenFallsBackToFile(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "zendesk-mgmt", "auth.json")
	if err := WriteFileConfig(path, FileConfig{Email: "file@example.com", APIToken: "file-token", AuthType: AuthTypeAPIToken}); err != nil {
		t.Fatalf("WriteFileConfig() error = %v", err)
	}

	resolver := NewResolver(Runtime{
		GOOS:          "windows",
		UserConfigDir: func() (string, error) { return tempDir, nil },
		Getenv:        func(string) string { return "" },
	}, nil)

	got, err := resolver.ResolveToken(ResolveOptions{Source: SourceEnvOrFile})
	if err != nil {
		t.Fatalf("ResolveToken() error = %v", err)
	}
	if got.Token != "file-token" {
		t.Fatalf("ResolveToken() token = %q, want %q", got.Token, "file-token")
	}
	if got.Email != "file@example.com" {
		t.Fatalf("ResolveToken() email = %q, want %q", got.Email, "file@example.com")
	}
	if got.ResolvedFrom != "file" {
		t.Fatalf("ResolveToken() resolvedFrom = %q, want file", got.ResolvedFrom)
	}
}

func TestResolveTokenFromKeychain(t *testing.T) {
	secret, err := encodeKeychainCredentials(storedCredentials{
		Email:    "keychain@example.com",
		Token:    "keychain-token",
		AuthType: AuthTypeAPIToken,
	})
	if err != nil {
		t.Fatalf("encodeKeychainCredentials() error = %v", err)
	}

	store := &fakeKeychainStore{token: secret}
	resolver := NewResolver(Runtime{
		GOOS:          "darwin",
		UserConfigDir: func() (string, error) { return t.TempDir(), nil },
		Getenv:        func(string) string { return "" },
	}, store)

	got, err := resolver.ResolveToken(ResolveOptions{
		Source:      SourceKeychain,
		InstanceURL: "https://acme.zendesk.com",
	})
	if err != nil {
		t.Fatalf("ResolveToken() error = %v", err)
	}
	if got.Token != "keychain-token" {
		t.Fatalf("ResolveToken() token = %q, want %q", got.Token, "keychain-token")
	}
	if got.Email != "keychain@example.com" {
		t.Fatalf("ResolveToken() email = %q, want %q", got.Email, "keychain@example.com")
	}
	if store.service != KeychainServiceName {
		t.Fatalf("keychain service = %q, want %q", store.service, KeychainServiceName)
	}
	if store.user != "https://acme.zendesk.com" {
		t.Fatalf("keychain user = %q, want instance URL", store.user)
	}
}

func TestResolveTokenFromKeychainBySuffix(t *testing.T) {
	secret, err := encodeKeychainCredentials(storedCredentials{
		Email:    "keychain@example.com",
		Token:    "keychain-token",
		AuthType: AuthTypeAPIToken,
	})
	if err != nil {
		t.Fatalf("encodeKeychainCredentials() error = %v", err)
	}

	store := &fakeKeychainStore{token: secret}
	resolver := NewResolver(Runtime{
		GOOS:          "darwin",
		UserConfigDir: func() (string, error) { return t.TempDir(), nil },
		Getenv:        func(string) string { return "" },
	}, store)

	got, err := resolver.ResolveToken(ResolveOptions{
		Source:    SourceKeychain,
		OrgSuffix: "Acme",
	})
	if err != nil {
		t.Fatalf("ResolveToken() error = %v", err)
	}
	if got.Token != "keychain-token" {
		t.Fatalf("ResolveToken() token = %q, want %q", got.Token, "keychain-token")
	}
	if store.user != "https://acme.zendesk.com" {
		t.Fatalf("keychain user = %q, want %q", store.user, "https://acme.zendesk.com")
	}
}

func TestResolveTokenKeychainRequiresInstanceURL(t *testing.T) {
	secret, err := encodeKeychainCredentials(storedCredentials{
		Email:    "keychain@example.com",
		Token:    "keychain-token",
		AuthType: AuthTypeAPIToken,
	})
	if err != nil {
		t.Fatalf("encodeKeychainCredentials() error = %v", err)
	}

	resolver := NewResolver(Runtime{
		GOOS:          "darwin",
		UserConfigDir: func() (string, error) { return t.TempDir(), nil },
		Getenv:        func(string) string { return "" },
	}, &fakeKeychainStore{token: secret})

	_, err = resolver.ResolveToken(ResolveOptions{Source: SourceKeychain})
	if !errors.Is(err, ErrInstanceURLRequired) {
		t.Fatalf("ResolveToken() error = %v, want %v", err, ErrInstanceURLRequired)
	}
}

func TestWriteAndReadFileConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zendesk-mgmt", "auth.json")
	want := FileConfig{Email: "test@example.com", APIToken: "test-token", AuthType: AuthTypeAPIToken}

	if err := WriteFileConfig(path, want); err != nil {
		t.Fatalf("WriteFileConfig() error = %v", err)
	}

	got, err := ReadFileConfig(path)
	if err != nil {
		t.Fatalf("ReadFileConfig() error = %v", err)
	}
	if got.APIToken != want.APIToken {
		t.Fatalf("ReadFileConfig() apiToken = %#v, want %#v", got.APIToken, want.APIToken)
	}
	if got.Email != want.Email {
		t.Fatalf("ReadFileConfig() email = %#v, want %#v", got.Email, want.Email)
	}
	if len(got.Profiles) != 0 {
		t.Fatalf("ReadFileConfig() profiles = %#v, want empty", got.Profiles)
	}
}

func TestSetAccessWritesProfileIntoFileConfig(t *testing.T) {
	tempDir := t.TempDir()
	resolver := NewResolver(Runtime{
		GOOS:          "windows",
		UserConfigDir: func() (string, error) { return tempDir, nil },
		Getenv:        func(string) string { return "" },
	}, nil)

	result, err := resolver.SetAccess(SetAccessOptions{
		Source:    SourceEnvOrFile,
		OrgSuffix: "Acme",
		Email:     "user@example.com",
		Token:     "file-token",
	})
	if err != nil {
		t.Fatalf("SetAccess() error = %v", err)
	}
	if result.StoredIn != "file" {
		t.Fatalf("SetAccess() storedIn = %q, want file", result.StoredIn)
	}

	cfg, err := ReadFileConfig(result.ConfigPath)
	if err != nil {
		t.Fatalf("ReadFileConfig() error = %v", err)
	}
	if got := cfg.Profiles["acme"].APIToken; got != "file-token" {
		t.Fatalf("profile token = %q, want %q", got, "file-token")
	}
	if got := cfg.Profiles["acme"].Email; got != "user@example.com" {
		t.Fatalf("profile email = %q, want %q", got, "user@example.com")
	}
}

func TestResolveTokenFromProfileInFileConfig(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "zendesk-mgmt", "auth.json")
	if err := WriteFileConfig(path, FileConfig{
		Profiles: map[string]FileProfile{
			"acme": {Email: "user@example.com", APIToken: "file-token", AuthType: AuthTypeAPIToken},
		},
	}); err != nil {
		t.Fatalf("WriteFileConfig() error = %v", err)
	}

	resolver := NewResolver(Runtime{
		GOOS:          "windows",
		UserConfigDir: func() (string, error) { return tempDir, nil },
		Getenv:        func(string) string { return "" },
	}, nil)

	got, err := resolver.ResolveToken(ResolveOptions{
		Source:    SourceEnvOrFile,
		OrgSuffix: "ACME",
	})
	if err != nil {
		t.Fatalf("ResolveToken() error = %v", err)
	}
	if got.Token != "file-token" {
		t.Fatalf("ResolveToken() token = %q, want %q", got.Token, "file-token")
	}
	if got.Email != "user@example.com" {
		t.Fatalf("ResolveToken() email = %q, want %q", got.Email, "user@example.com")
	}
}

func TestSetAccessWritesIntoKeychainBySuffix(t *testing.T) {
	store := &fakeKeychainStore{}
	resolver := NewResolver(Runtime{
		GOOS:          "darwin",
		UserConfigDir: func() (string, error) { return t.TempDir(), nil },
		Getenv:        func(string) string { return "" },
	}, store)

	result, err := resolver.SetAccess(SetAccessOptions{
		Source:    SourceKeychain,
		OrgSuffix: "Acme",
		Email:     "user@example.com",
		Token:     "keychain-token",
	})
	if err != nil {
		t.Fatalf("SetAccess() error = %v", err)
	}
	if result.InstanceURL != "https://acme.zendesk.com" {
		t.Fatalf("SetAccess() instanceURL = %q, want %q", result.InstanceURL, "https://acme.zendesk.com")
	}
	if len(store.sets) != 1 {
		t.Fatalf("keychain set count = %d, want 1", len(store.sets))
	}
	got := store.sets[0]
	if got.service != KeychainServiceName {
		t.Fatalf("keychain service = %q, want %q", got.service, KeychainServiceName)
	}
	if got.user != "https://acme.zendesk.com" {
		t.Fatalf("keychain user = %q, want %q", got.user, "https://acme.zendesk.com")
	}
	if !strings.Contains(got.password, "\"email\":\"user@example.com\"") {
		t.Fatalf("keychain password missing email payload: %q", got.password)
	}
	if !strings.Contains(got.password, "\"api_token\":\"keychain-token\"") {
		t.Fatalf("keychain password missing token payload: %q", got.password)
	}
}

func TestNormalizeSuffix(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "Acme", want: "acme"},
		{in: "https://acme.zendesk.com", want: "acme"},
		{in: "acme.zendesk.com", want: "acme"},
		{in: "acme/", want: "acme"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := NormalizeSuffix(tt.in); got != tt.want {
				t.Fatalf("NormalizeSuffix(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestInspectAccessForFileProfile(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "zendesk-mgmt", "auth.json")
	if err := WriteFileConfig(path, FileConfig{
		Profiles: map[string]FileProfile{
			"acme": {Email: "user@example.com", APIToken: "file-token", AuthType: AuthTypeAPIToken},
			"beta": {Email: "beta@example.com", APIToken: "beta-token", AuthType: AuthTypeAPIToken},
		},
	}); err != nil {
		t.Fatalf("WriteFileConfig() error = %v", err)
	}

	resolver := NewResolver(Runtime{
		GOOS:          "windows",
		UserConfigDir: func() (string, error) { return tempDir, nil },
		Getenv:        func(string) string { return "" },
	}, nil)

	got, err := resolver.InspectAccess(ResolveOptions{
		Source:    SourceEnvOrFile,
		OrgSuffix: "acme",
	})
	if err != nil {
		t.Fatalf("InspectAccess() error = %v", err)
	}
	if !got.AccessTokenPresent {
		t.Fatal("InspectAccess() accessTokenPresent = false, want true")
	}
	if got.Email != "user@example.com" {
		t.Fatalf("InspectAccess() email = %q, want %q", got.Email, "user@example.com")
	}
	if len(got.AvailableProfiles) != 2 {
		t.Fatalf("InspectAccess() availableProfiles = %#v, want 2 profiles", got.AvailableProfiles)
	}
}

func TestInspectAccessForKeychainBySuffix(t *testing.T) {
	secret, err := encodeKeychainCredentials(storedCredentials{
		Email:    "keychain@example.com",
		Token:    "keychain-token",
		AuthType: AuthTypeAPIToken,
	})
	if err != nil {
		t.Fatalf("encodeKeychainCredentials() error = %v", err)
	}

	store := &fakeKeychainStore{token: secret}
	resolver := NewResolver(Runtime{
		GOOS:          "darwin",
		UserConfigDir: func() (string, error) { return t.TempDir(), nil },
		Getenv:        func(string) string { return "" },
	}, store)

	got, err := resolver.InspectAccess(ResolveOptions{
		Source:    SourceKeychain,
		OrgSuffix: "acme",
	})
	if err != nil {
		t.Fatalf("InspectAccess() error = %v", err)
	}
	if got.AccountKey != "https://acme.zendesk.com" {
		t.Fatalf("InspectAccess() accountKey = %q, want %q", got.AccountKey, "https://acme.zendesk.com")
	}
	if !got.AccessTokenPresent {
		t.Fatal("InspectAccess() accessTokenPresent = false, want true")
	}
	if got.Email != "keychain@example.com" {
		t.Fatalf("InspectAccess() email = %q, want %q", got.Email, "keychain@example.com")
	}
}

func TestClearAccessRemovesFileProfile(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "zendesk-mgmt", "auth.json")
	if err := WriteFileConfig(path, FileConfig{
		Profiles: map[string]FileProfile{
			"acme": {Email: "user@example.com", APIToken: "file-token", AuthType: AuthTypeAPIToken},
			"beta": {Email: "beta@example.com", APIToken: "beta-token", AuthType: AuthTypeAPIToken},
		},
	}); err != nil {
		t.Fatalf("WriteFileConfig() error = %v", err)
	}

	resolver := NewResolver(Runtime{
		GOOS:          "windows",
		UserConfigDir: func() (string, error) { return tempDir, nil },
		Getenv:        func(string) string { return "" },
	}, nil)

	result, err := resolver.ClearAccess(ResolveOptions{
		Source:    SourceEnvOrFile,
		OrgSuffix: "acme",
	})
	if err != nil {
		t.Fatalf("ClearAccess() error = %v", err)
	}
	if !result.Removed {
		t.Fatal("ClearAccess() removed = false, want true")
	}

	cfg, err := ReadFileConfig(path)
	if err != nil {
		t.Fatalf("ReadFileConfig() error = %v", err)
	}
	if _, ok := cfg.Profiles["acme"]; ok {
		t.Fatalf("profile acme still present: %#v", cfg.Profiles)
	}
	if got := cfg.Profiles["beta"].APIToken; got != "beta-token" {
		t.Fatalf("profile beta token = %q, want %q", got, "beta-token")
	}
}

func TestClearAccessDeletesFileWhenLastProfileRemoved(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "zendesk-mgmt", "auth.json")
	if err := WriteFileConfig(path, FileConfig{
		Profiles: map[string]FileProfile{
			"acme": {Email: "user@example.com", APIToken: "file-token", AuthType: AuthTypeAPIToken},
		},
	}); err != nil {
		t.Fatalf("WriteFileConfig() error = %v", err)
	}

	resolver := NewResolver(Runtime{
		GOOS:          "windows",
		UserConfigDir: func() (string, error) { return tempDir, nil },
		Getenv:        func(string) string { return "" },
	}, nil)

	_, err := resolver.ClearAccess(ResolveOptions{
		Source:    SourceEnvOrFile,
		OrgSuffix: "acme",
	})
	if err != nil {
		t.Fatalf("ClearAccess() error = %v", err)
	}

	if _, err := ReadFileConfig(path); !errors.Is(err, ErrAccessTokenNotFound) {
		t.Fatalf("ReadFileConfig() error = %v, want %v", err, ErrAccessTokenNotFound)
	}
}

func TestClearAccessDeletesKeychainEntry(t *testing.T) {
	store := &fakeKeychainStore{}
	resolver := NewResolver(Runtime{
		GOOS:          "darwin",
		UserConfigDir: func() (string, error) { return t.TempDir(), nil },
		Getenv:        func(string) string { return "" },
	}, store)

	result, err := resolver.ClearAccess(ResolveOptions{
		Source:    SourceKeychain,
		OrgSuffix: "acme",
	})
	if err != nil {
		t.Fatalf("ClearAccess() error = %v", err)
	}
	if !result.Removed {
		t.Fatal("ClearAccess() removed = false, want true")
	}
	if len(store.deletes) != 1 {
		t.Fatalf("delete count = %d, want 1", len(store.deletes))
	}
	if got := store.deletes[0].user; got != "https://acme.zendesk.com" {
		t.Fatalf("delete user = %q, want %q", got, "https://acme.zendesk.com")
	}
}

func TestResolveTokenEnvRequiresEmail(t *testing.T) {
	resolver := NewResolver(Runtime{
		GOOS:          "windows",
		UserConfigDir: func() (string, error) { return t.TempDir(), nil },
		Getenv: func(key string) string {
			if key == APITokenEnvVar {
				return "env-token"
			}
			return ""
		},
	}, nil)

	_, err := resolver.ResolveToken(ResolveOptions{Source: SourceEnvOrFile})
	if !errors.Is(err, ErrEmailRequired) {
		t.Fatalf("ResolveToken() error = %v, want %v", err, ErrEmailRequired)
	}
}
