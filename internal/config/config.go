package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

const (
	KeychainServiceName          = "zendesk-mgmt"
	AccessTokenEnvVar            = "ZENDESK_ACCESS_TOKEN"
	APITokenEnvVar               = "ZENDESK_API_TOKEN"
	EmailEnvVar                  = "ZENDESK_EMAIL"
	DefaultAuthType     AuthType = AuthTypeAPIToken
)

type Source string
type AuthType string

const (
	SourceAuto      Source = "auto"
	SourceKeychain  Source = "keychain"
	SourceEnvOrFile Source = "env_or_file"

	AuthTypeAPIToken AuthType = "api_token"
)

type FileConfig struct {
	AccessToken string                 `json:"access_token,omitempty"`
	APIToken    string                 `json:"api_token,omitempty"`
	Email       string                 `json:"email,omitempty"`
	AuthType    AuthType               `json:"auth_type,omitempty"`
	Profiles    map[string]FileProfile `json:"profiles,omitempty"`
}

type FileProfile struct {
	AccessToken string   `json:"access_token,omitempty"`
	APIToken    string   `json:"api_token,omitempty"`
	Email       string   `json:"email,omitempty"`
	AuthType    AuthType `json:"auth_type,omitempty"`
}

type ResolveOptions struct {
	Source      Source
	InstanceURL string
	OrgSuffix   string
}

type SetAccessOptions struct {
	Source    Source
	OrgSuffix string
	Email     string
	Token     string
}

type SetAccessResult struct {
	Source      Source
	StoredIn    string
	ConfigPath  string
	OrgSuffix   string
	Email       string
	AuthType    AuthType
	InstanceURL string
	AccountKey  string
	SectionName string
}

type AccessStatus struct {
	Source             Source
	StoredIn           string
	ResolvedFrom       string
	ConfigPath         string
	OrgSuffix          string
	Email              string
	AuthType           AuthType
	InstanceURL        string
	AccountKey         string
	SectionName        string
	AccessTokenPresent bool
	AvailableProfiles  []string
}

type ClearAccessResult struct {
	Source      Source
	StoredIn    string
	ConfigPath  string
	OrgSuffix   string
	InstanceURL string
	AccountKey  string
	SectionName string
	Removed     bool
}

type ResolvedToken struct {
	Token        string
	Email        string
	AuthType     AuthType
	Source       Source
	ResolvedFrom string
	ConfigPath   string
}

type Runtime struct {
	GOOS          string
	UserConfigDir func() (string, error)
	Getenv        func(string) string
}

type KeychainStore interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
	Delete(service, user string) error
}

type Resolver struct {
	runtime  Runtime
	keychain KeychainStore
}

var (
	ErrAccessTokenNotFound  = errors.New("access token not found")
	ErrOrganizationRequired = errors.New("organization is required")
	ErrOrganizationNotFound = errors.New("organization profile not found")
	ErrInstanceURLRequired  = errors.New("instance URL is required for keychain mode")
	ErrOrgSuffixRequired    = errors.New("organization is required")
	ErrEmailRequired        = errors.New("email is required for api_token auth")
)

type OrganizationRequiredError struct {
	ConfigPath        string
	AvailableProfiles []string
}

func (e *OrganizationRequiredError) Error() string {
	var b strings.Builder
	b.WriteString("organization is required to select a stored profile")
	if len(e.AvailableProfiles) > 0 {
		b.WriteString("; available profiles: ")
		b.WriteString(strings.Join(e.AvailableProfiles, ", "))
	}
	if strings.TrimSpace(e.ConfigPath) != "" {
		b.WriteString(" (config: ")
		b.WriteString(e.ConfigPath)
		b.WriteString(")")
	}
	return b.String()
}

func (e *OrganizationRequiredError) Unwrap() error {
	return ErrOrganizationRequired
}

type OrganizationNotFoundError struct {
	Organization      string
	ConfigPath        string
	AvailableProfiles []string
}

func (e *OrganizationNotFoundError) Error() string {
	var b strings.Builder
	b.WriteString("organization profile ")
	b.WriteString(strconv.Quote(strings.TrimSpace(e.Organization)))
	b.WriteString(" was not found")
	if len(e.AvailableProfiles) > 0 {
		b.WriteString("; available profiles: ")
		b.WriteString(strings.Join(e.AvailableProfiles, ", "))
	}
	if strings.TrimSpace(e.ConfigPath) != "" {
		b.WriteString(" (config: ")
		b.WriteString(e.ConfigPath)
		b.WriteString(")")
	}
	return b.String()
}

func (e *OrganizationNotFoundError) Unwrap() error {
	return ErrOrganizationNotFound
}

func NewResolver(rt Runtime, keychain KeychainStore) *Resolver {
	if rt.GOOS == "" {
		rt.GOOS = runtime.GOOS
	}
	if rt.UserConfigDir == nil {
		rt.UserConfigDir = os.UserConfigDir
	}
	if rt.Getenv == nil {
		rt.Getenv = os.Getenv
	}

	return &Resolver{
		runtime:  rt,
		keychain: keychain,
	}
}

func DefaultSourceForGOOS(goos string) Source {
	if strings.EqualFold(goos, "darwin") {
		return SourceKeychain
	}
	return SourceEnvOrFile
}

func (r *Resolver) AuthConfigPath() (string, error) {
	configDir, err := r.runtime.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(configDir, "zendesk-mgmt", "auth.json"), nil
}

func (r *Resolver) ResolveToken(opts ResolveOptions) (ResolvedToken, error) {
	source := opts.Source
	if source == "" || source == SourceAuto {
		source = DefaultSourceForGOOS(r.runtime.GOOS)
	}

	switch source {
	case SourceKeychain:
		return r.resolveFromKeychain(opts.InstanceURL, opts.OrgSuffix)
	case SourceEnvOrFile:
		return r.resolveFromEnvOrFile(opts.OrgSuffix)
	default:
		return ResolvedToken{}, fmt.Errorf("unsupported source %q", source)
	}
}

func (r *Resolver) resolveFromKeychain(instanceURL, orgSuffix string) (ResolvedToken, error) {
	accountKey, resolvedInstanceURL, err := accountKey(instanceURL, orgSuffix)
	if err != nil {
		return ResolvedToken{}, err
	}
	if strings.TrimSpace(resolvedInstanceURL) == "" {
		return ResolvedToken{}, ErrInstanceURLRequired
	}
	if r.keychain == nil {
		return ResolvedToken{}, fmt.Errorf("keychain store is not configured")
	}

	token, err := r.keychain.Get(KeychainServiceName, accountKey)
	if err != nil {
		return ResolvedToken{}, fmt.Errorf("load access token from keychain: %w", err)
	}
	credentials := decodeKeychainCredentials(token)
	if strings.TrimSpace(credentials.Token) == "" {
		return ResolvedToken{}, ErrAccessTokenNotFound
	}
	if err := validateResolvedCredentials(credentials); err != nil {
		return ResolvedToken{}, err
	}

	return ResolvedToken{
		Token:        credentials.Token,
		Email:        credentials.Email,
		AuthType:     credentials.AuthType,
		Source:       SourceKeychain,
		ResolvedFrom: "keychain",
	}, nil
}

func (r *Resolver) resolveFromEnvOrFile(orgSuffix string) (ResolvedToken, error) {
	if envCreds := credentialsFromEnv(r.runtime.Getenv); strings.TrimSpace(envCreds.Token) != "" {
		if err := validateResolvedCredentials(envCreds); err != nil {
			return ResolvedToken{}, err
		}
		path, _ := r.AuthConfigPath()
		return ResolvedToken{
			Token:        envCreds.Token,
			Email:        envCreds.Email,
			AuthType:     envCreds.AuthType,
			Source:       SourceEnvOrFile,
			ResolvedFrom: "env",
			ConfigPath:   path,
		}, nil
	}

	path, err := r.AuthConfigPath()
	if err != nil {
		return ResolvedToken{}, err
	}

	cfg, err := ReadFileConfig(path)
	if err != nil {
		return ResolvedToken{}, err
	}

	if suffix := NormalizeSuffix(orgSuffix); suffix != "" {
		profile, ok := cfg.Profiles[suffix]
		credentials := credentialsFromFileProfile(profile)
		if !ok || strings.TrimSpace(credentials.Token) == "" {
			if available := profileNames(cfg.Profiles); len(available) > 0 {
				return ResolvedToken{}, &OrganizationNotFoundError{
					Organization:      suffix,
					ConfigPath:        path,
					AvailableProfiles: available,
				}
			}
			return ResolvedToken{}, ErrAccessTokenNotFound
		}
		if err := validateResolvedCredentials(credentials); err != nil {
			return ResolvedToken{}, err
		}
		return ResolvedToken{
			Token:        credentials.Token,
			Email:        credentials.Email,
			AuthType:     credentials.AuthType,
			Source:       SourceEnvOrFile,
			ResolvedFrom: "file",
			ConfigPath:   path,
		}, nil
	}

	credentials := credentialsFromFileConfig(cfg)
	if strings.TrimSpace(credentials.Token) == "" {
		if available := profileNames(cfg.Profiles); len(available) > 0 {
			return ResolvedToken{}, &OrganizationRequiredError{
				ConfigPath:        path,
				AvailableProfiles: available,
			}
		}
		return ResolvedToken{}, ErrAccessTokenNotFound
	}
	if err := validateResolvedCredentials(credentials); err != nil {
		return ResolvedToken{}, err
	}

	return ResolvedToken{
		Token:        credentials.Token,
		Email:        credentials.Email,
		AuthType:     credentials.AuthType,
		Source:       SourceEnvOrFile,
		ResolvedFrom: "file",
		ConfigPath:   path,
	}, nil
}

func (r *Resolver) SetAccess(opts SetAccessOptions) (SetAccessResult, error) {
	if strings.TrimSpace(opts.Token) == "" {
		return SetAccessResult{}, errors.New("token is required")
	}
	if strings.TrimSpace(opts.Email) == "" {
		return SetAccessResult{}, ErrEmailRequired
	}

	source := opts.Source
	if source == "" || source == SourceAuto {
		source = DefaultSourceForGOOS(r.runtime.GOOS)
	}

	switch source {
	case SourceKeychain:
		return r.setAccessKeychain(opts.OrgSuffix, opts.Email, opts.Token)
	case SourceEnvOrFile:
		return r.setAccessEnvOrFile(opts.OrgSuffix, opts.Email, opts.Token)
	default:
		return SetAccessResult{}, fmt.Errorf("unsupported source %q", source)
	}
}

func (r *Resolver) InspectAccess(opts ResolveOptions) (AccessStatus, error) {
	source := opts.Source
	if source == "" || source == SourceAuto {
		source = DefaultSourceForGOOS(r.runtime.GOOS)
	}

	switch source {
	case SourceKeychain:
		return r.inspectKeychain(opts.InstanceURL, opts.OrgSuffix)
	case SourceEnvOrFile:
		return r.inspectEnvOrFile(opts.OrgSuffix)
	default:
		return AccessStatus{}, fmt.Errorf("unsupported source %q", source)
	}
}

func (r *Resolver) ClearAccess(opts ResolveOptions) (ClearAccessResult, error) {
	source := opts.Source
	if source == "" || source == SourceAuto {
		source = DefaultSourceForGOOS(r.runtime.GOOS)
	}

	switch source {
	case SourceKeychain:
		return r.clearAccessKeychain(opts.InstanceURL, opts.OrgSuffix)
	case SourceEnvOrFile:
		return r.clearAccessEnvOrFile(opts.OrgSuffix)
	default:
		return ClearAccessResult{}, fmt.Errorf("unsupported source %q", source)
	}
}

func (r *Resolver) setAccessKeychain(orgSuffix, email, token string) (SetAccessResult, error) {
	if r.keychain == nil {
		return SetAccessResult{}, fmt.Errorf("keychain store is not configured")
	}

	suffix := NormalizeSuffix(orgSuffix)
	if suffix == "" {
		return SetAccessResult{}, ErrOrgSuffixRequired
	}

	instanceURL := InstanceURLFromSuffix(suffix)
	accountKey, _, err := accountKey(instanceURL, "")
	if err != nil {
		return SetAccessResult{}, err
	}

	secret, err := encodeKeychainCredentials(storedCredentials{
		Email:    email,
		Token:    token,
		AuthType: DefaultAuthType,
	})
	if err != nil {
		return SetAccessResult{}, err
	}

	if err := r.keychain.Set(KeychainServiceName, accountKey, secret); err != nil {
		return SetAccessResult{}, fmt.Errorf("store access token in keychain: %w", err)
	}

	return SetAccessResult{
		Source:      SourceKeychain,
		StoredIn:    "keychain",
		OrgSuffix:   suffix,
		Email:       strings.TrimSpace(email),
		AuthType:    DefaultAuthType,
		InstanceURL: instanceURL,
		AccountKey:  accountKey,
	}, nil
}

func (r *Resolver) setAccessEnvOrFile(orgSuffix, email, token string) (SetAccessResult, error) {
	suffix := NormalizeSuffix(orgSuffix)
	if suffix == "" {
		return SetAccessResult{}, ErrOrgSuffixRequired
	}

	path, err := r.AuthConfigPath()
	if err != nil {
		return SetAccessResult{}, err
	}

	cfg, err := ReadFileConfig(path)
	if err != nil && !errors.Is(err, ErrAccessTokenNotFound) {
		return SetAccessResult{}, err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]FileProfile{}
	}
	cfg.Profiles[suffix] = FileProfile{
		Email:    strings.TrimSpace(email),
		APIToken: strings.TrimSpace(token),
		AuthType: DefaultAuthType,
	}

	if err := WriteFileConfig(path, cfg); err != nil {
		return SetAccessResult{}, err
	}

	return SetAccessResult{
		Source:      SourceEnvOrFile,
		StoredIn:    "file",
		ConfigPath:  path,
		OrgSuffix:   suffix,
		Email:       strings.TrimSpace(email),
		AuthType:    DefaultAuthType,
		SectionName: suffix,
	}, nil
}

func (r *Resolver) inspectKeychain(instanceURL, orgSuffix string) (AccessStatus, error) {
	accountKey, resolvedInstanceURL, err := accountKey(instanceURL, orgSuffix)
	if err != nil {
		return AccessStatus{}, err
	}
	if r.keychain == nil {
		return AccessStatus{}, fmt.Errorf("keychain store is not configured")
	}

	secret, err := r.keychain.Get(KeychainServiceName, accountKey)
	if err != nil && !isProbablyMissingSecret(err) {
		return AccessStatus{}, fmt.Errorf("inspect keychain access token: %w", err)
	}
	credentials := decodeKeychainCredentials(secret)

	return AccessStatus{
		Source:             SourceKeychain,
		StoredIn:           "keychain",
		OrgSuffix:          NormalizeSuffix(orgSuffix),
		Email:              credentials.Email,
		AuthType:           credentials.AuthType,
		InstanceURL:        resolvedInstanceURL,
		AccountKey:         accountKey,
		AccessTokenPresent: err == nil && strings.TrimSpace(credentials.Token) != "",
	}, nil
}

func (r *Resolver) inspectEnvOrFile(orgSuffix string) (AccessStatus, error) {
	path, err := r.AuthConfigPath()
	if err != nil {
		return AccessStatus{}, err
	}

	suffix := NormalizeSuffix(orgSuffix)
	if envCreds := credentialsFromEnv(r.runtime.Getenv); strings.TrimSpace(envCreds.Token) != "" {
		return AccessStatus{
			Source:             SourceEnvOrFile,
			StoredIn:           "env_or_file",
			ResolvedFrom:       "env",
			ConfigPath:         path,
			OrgSuffix:          suffix,
			Email:              envCreds.Email,
			AuthType:           envCreds.AuthType,
			InstanceURL:        InstanceURLFromSuffix(suffix),
			SectionName:        suffix,
			AccessTokenPresent: true,
		}, nil
	}

	cfg, err := ReadFileConfig(path)
	if err != nil && !errors.Is(err, ErrAccessTokenNotFound) {
		return AccessStatus{}, err
	}

	status := AccessStatus{
		Source:            SourceEnvOrFile,
		StoredIn:          "file",
		ConfigPath:        path,
		OrgSuffix:         suffix,
		AuthType:          DefaultAuthType,
		InstanceURL:       InstanceURLFromSuffix(suffix),
		SectionName:       suffix,
		AvailableProfiles: profileNames(cfg.Profiles),
	}

	if suffix != "" {
		credentials := credentialsFromFileProfile(cfg.Profiles[suffix])
		status.Email = credentials.Email
		status.AuthType = credentials.AuthType
		if strings.TrimSpace(credentials.Token) != "" {
			status.AccessTokenPresent = true
		}
		return status, nil
	}

	credentials := credentialsFromFileConfig(cfg)
	status.Email = credentials.Email
	status.AuthType = credentials.AuthType
	status.AccessTokenPresent = strings.TrimSpace(credentials.Token) != ""
	return status, nil
}

func (r *Resolver) clearAccessKeychain(instanceURL, orgSuffix string) (ClearAccessResult, error) {
	accountKey, resolvedInstanceURL, err := accountKey(instanceURL, orgSuffix)
	if err != nil {
		return ClearAccessResult{}, err
	}
	if r.keychain == nil {
		return ClearAccessResult{}, fmt.Errorf("keychain store is not configured")
	}

	err = r.keychain.Delete(KeychainServiceName, accountKey)
	if err != nil && !isProbablyMissingSecret(err) {
		return ClearAccessResult{}, fmt.Errorf("delete keychain access token: %w", err)
	}

	return ClearAccessResult{
		Source:      SourceKeychain,
		StoredIn:    "keychain",
		OrgSuffix:   NormalizeSuffix(orgSuffix),
		InstanceURL: resolvedInstanceURL,
		AccountKey:  accountKey,
		Removed:     err == nil,
	}, nil
}

func (r *Resolver) clearAccessEnvOrFile(orgSuffix string) (ClearAccessResult, error) {
	path, err := r.AuthConfigPath()
	if err != nil {
		return ClearAccessResult{}, err
	}

	cfg, err := ReadFileConfig(path)
	if err != nil {
		if errors.Is(err, ErrAccessTokenNotFound) {
			return ClearAccessResult{
				Source:      SourceEnvOrFile,
				StoredIn:    "file",
				ConfigPath:  path,
				OrgSuffix:   NormalizeSuffix(orgSuffix),
				SectionName: NormalizeSuffix(orgSuffix),
				Removed:     false,
			}, nil
		}
		return ClearAccessResult{}, err
	}

	suffix := NormalizeSuffix(orgSuffix)
	removed := false
	if suffix != "" {
		if _, ok := cfg.Profiles[suffix]; ok {
			delete(cfg.Profiles, suffix)
			removed = true
		}
	} else if strings.TrimSpace(cfg.AccessToken) != "" {
		cfg.AccessToken = ""
		removed = true
	}

	if removed {
		if configHasAnyTokens(cfg) {
			if err := WriteFileConfig(path, cfg); err != nil {
				return ClearAccessResult{}, err
			}
		} else if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return ClearAccessResult{}, fmt.Errorf("remove empty auth config: %w", err)
		}
	}

	return ClearAccessResult{
		Source:      SourceEnvOrFile,
		StoredIn:    "file",
		ConfigPath:  path,
		OrgSuffix:   suffix,
		SectionName: suffix,
		Removed:     removed,
	}, nil
}

func ReadFileConfig(path string) (FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return FileConfig{}, ErrAccessTokenNotFound
		}
		return FileConfig{}, fmt.Errorf("read auth config: %w", err)
	}

	var cfg FileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return FileConfig{}, fmt.Errorf("decode auth config: %w", err)
	}

	return cfg, nil
}

func WriteFileConfig(path string, cfg FileConfig) error {
	if !configHasAnyTokens(cfg) {
		return errors.New("at least one access token is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode auth config: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write auth config: %w", err)
	}
	return nil
}

func configHasAnyTokens(cfg FileConfig) bool {
	if strings.TrimSpace(cfg.AccessToken) != "" || strings.TrimSpace(cfg.APIToken) != "" {
		return true
	}
	for _, profile := range cfg.Profiles {
		if strings.TrimSpace(profile.AccessToken) != "" || strings.TrimSpace(profile.APIToken) != "" {
			return true
		}
	}
	return false
}

func NormalizeSuffix(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	value = strings.TrimSuffix(value, ".zendesk.com")
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimSuffix(value, "/")
	if idx := strings.IndexByte(value, '.'); idx >= 0 {
		value = value[:idx]
	}
	return value
}

func InstanceURLFromSuffix(suffix string) string {
	normalized := NormalizeSuffix(suffix)
	if normalized == "" {
		return ""
	}
	return "https://" + normalized + ".zendesk.com"
}

func accountKey(instanceURL, orgSuffix string) (string, string, error) {
	resolvedInstanceURL := strings.TrimSpace(instanceURL)
	if resolvedInstanceURL == "" {
		resolvedInstanceURL = InstanceURLFromSuffix(orgSuffix)
	}
	if resolvedInstanceURL == "" {
		return "", "", ErrInstanceURLRequired
	}
	return resolvedInstanceURL, resolvedInstanceURL, nil
}

func profileNames(profiles map[string]FileProfile) []string {
	if len(profiles) == 0 {
		return nil
	}

	names := make([]string, 0, len(profiles))
	for name, profile := range profiles {
		if strings.TrimSpace(profile.AccessToken) == "" && strings.TrimSpace(profile.APIToken) == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func isProbablyMissingSecret(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") || strings.Contains(msg, "could not be found")
}

type storedCredentials struct {
	Email    string   `json:"email,omitempty"`
	Token    string   `json:"api_token,omitempty"`
	AuthType AuthType `json:"auth_type,omitempty"`
}

func credentialsFromEnv(getenv func(string) string) storedCredentials {
	token := strings.TrimSpace(getenv(APITokenEnvVar))
	if token == "" {
		token = strings.TrimSpace(getenv(AccessTokenEnvVar))
	}

	return storedCredentials{
		Email:    strings.TrimSpace(getenv(EmailEnvVar)),
		Token:    token,
		AuthType: normalizedAuthType(DefaultAuthType),
	}
}

func credentialsFromFileConfig(cfg FileConfig) storedCredentials {
	token := strings.TrimSpace(cfg.APIToken)
	if token == "" {
		token = strings.TrimSpace(cfg.AccessToken)
	}

	return storedCredentials{
		Email:    strings.TrimSpace(cfg.Email),
		Token:    token,
		AuthType: normalizedAuthType(cfg.AuthType),
	}
}

func credentialsFromFileProfile(profile FileProfile) storedCredentials {
	token := strings.TrimSpace(profile.APIToken)
	if token == "" {
		token = strings.TrimSpace(profile.AccessToken)
	}

	return storedCredentials{
		Email:    strings.TrimSpace(profile.Email),
		Token:    token,
		AuthType: normalizedAuthType(profile.AuthType),
	}
}

func normalizedAuthType(value AuthType) AuthType {
	if strings.TrimSpace(string(value)) == "" {
		return DefaultAuthType
	}
	return value
}

func validateResolvedCredentials(credentials storedCredentials) error {
	if strings.TrimSpace(credentials.Token) == "" {
		return ErrAccessTokenNotFound
	}
	if normalizedAuthType(credentials.AuthType) == AuthTypeAPIToken && strings.TrimSpace(credentials.Email) == "" {
		return ErrEmailRequired
	}
	return nil
}

func encodeKeychainCredentials(credentials storedCredentials) (string, error) {
	payload := storedCredentials{
		Email:    strings.TrimSpace(credentials.Email),
		Token:    strings.TrimSpace(credentials.Token),
		AuthType: normalizedAuthType(credentials.AuthType),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode keychain credentials: %w", err)
	}
	return string(data), nil
}

func decodeKeychainCredentials(secret string) storedCredentials {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return storedCredentials{}
	}

	var raw struct {
		Email       string   `json:"email,omitempty"`
		APIToken    string   `json:"api_token,omitempty"`
		AccessToken string   `json:"access_token,omitempty"`
		AuthType    AuthType `json:"auth_type,omitempty"`
	}
	if err := json.Unmarshal([]byte(secret), &raw); err == nil {
		token := strings.TrimSpace(raw.APIToken)
		if token == "" {
			token = strings.TrimSpace(raw.AccessToken)
		}
		return storedCredentials{
			Email:    strings.TrimSpace(raw.Email),
			Token:    token,
			AuthType: normalizedAuthType(raw.AuthType),
		}
	}

	return storedCredentials{
		Token:    secret,
		AuthType: DefaultAuthType,
	}
}
