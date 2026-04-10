package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/relux-works/skill-zendesk-management/internal/config"
	"github.com/relux-works/skill-zendesk-management/internal/zendesk"
	"github.com/zalando/go-keyring"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "auth":
		runAuth(os.Args[2:])
	case "q":
		runQuery(os.Args[2:])
	case "grep":
		runGrep(os.Args[2:])
	case "attachment":
		runAttachment(os.Args[2:])
	case "version", "--version":
		printVersion()
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  zendesk-mgmt version")
	fmt.Fprintln(os.Stderr, "  zendesk-mgmt q '<query>' --organization ORG --format json|compact [--source auto|keychain|env_or_file] [--instance URL]")
	fmt.Fprintln(os.Stderr, "  zendesk-mgmt grep 'text query' --organization ORG --format json|compact [--type ticket|user|organization] [--limit N]")
	fmt.Fprintln(os.Stderr, "  zendesk-mgmt attachment download ATTACHMENT_ID --organization ORG [--destination PATH] [--force] [--source auto|keychain|env_or_file] [--instance URL]")
	fmt.Fprintln(os.Stderr, "  zendesk-mgmt auth config-path")
	fmt.Fprintln(os.Stderr, "  zendesk-mgmt auth set-access --organization ORG --email EMAIL --token TOKEN [--source auto|keychain|env_or_file]")
	fmt.Fprintln(os.Stderr, "  zendesk-mgmt auth whoami [--source auto|keychain|env_or_file] [--organization ORG] [--instance URL] [--check=false]")
	fmt.Fprintln(os.Stderr, "  zendesk-mgmt auth clean [--source auto|keychain|env_or_file] [--organization ORG] [--instance URL]")
	fmt.Fprintln(os.Stderr, "  zendesk-mgmt auth clear-access [--source auto|keychain|env_or_file] [--organization ORG] [--instance URL]")
	fmt.Fprintln(os.Stderr, "  zendesk-mgmt auth write-config --organization ORG --email EMAIL --token TOKEN [--path PATH]  # low-level")
	fmt.Fprintln(os.Stderr, "  zendesk-mgmt auth resolve [--source auto|keychain|env_or_file] [--organization ORG] [--instance URL]")
}

func printVersion() {
	fmt.Printf("zendesk-mgmt %s commit=%s build_date=%s %s/%s\n", Version, Commit, BuildDate, runtime.GOOS, runtime.GOARCH)
}

func runAuth(args []string) {
	if len(args) < 1 {
		usage()
		os.Exit(2)
	}

	resolver := config.NewResolver(
		config.Runtime{
			GOOS:          runtime.GOOS,
			UserConfigDir: os.UserConfigDir,
			Getenv:        os.Getenv,
		},
		config.NewKeychainStore(keyring.Get, keyring.Set, keyring.Delete),
	)

	switch args[0] {
	case "config-path":
		path, err := resolver.AuthConfigPath()
		if err != nil {
			fatalErr(err)
		}
		fmt.Println(path)
	case "set-access":
		runAuthSetAccess(args[1:], resolver)
	case "whoami":
		runAuthWhoAmI(args[1:], resolver)
	case "clean", "cleanup", "clear":
		runAuthClearAccess(args[1:], resolver)
	case "clear-access":
		runAuthClearAccess(args[1:], resolver)
	case "write-config":
		runAuthWriteConfig(args[1:], resolver)
	case "resolve":
		runAuthResolve(args[1:], resolver)
	default:
		fmt.Fprintf(os.Stderr, "unknown auth command: %s\n\n", args[0])
		usage()
		os.Exit(2)
	}
}

func runQuery(args []string) {
	fs := flag.NewFlagSet("q", flag.ExitOnError)
	source := fs.String("source", string(config.SourceAuto), "Token source: auto, keychain, env_or_file")
	organization := bindOrganizationFlags(fs)
	instance := fs.String("instance", "", "Zendesk instance URL override")
	format := fs.String("format", "json", "Output format: json or compact")
	_ = fs.Parse(reorderFlagArgs(args))

	if fs.NArg() != 1 {
		fatalErr(fmt.Errorf("q requires exactly one query string argument"))
	}

	resolver := config.NewResolver(
		config.Runtime{
			GOOS:          runtime.GOOS,
			UserConfigDir: os.UserConfigDir,
			Getenv:        os.Getenv,
		},
		config.NewKeychainStore(keyring.Get, keyring.Set, keyring.Delete),
	)

	client, err := newZendeskClientFromResolver(resolver, config.Source(*source), *organization, *instance)
	if err != nil {
		fatalErr(err)
	}

	results, err := zendesk.NewQueryEngine(client).Execute(context.Background(), fs.Arg(0))
	if err != nil {
		fatalErr(err)
	}

	if err := writeResults(*format, results); err != nil {
		fatalErr(err)
	}
}

func runGrep(args []string) {
	fs := flag.NewFlagSet("grep", flag.ExitOnError)
	source := fs.String("source", string(config.SourceAuto), "Token source: auto, keychain, env_or_file")
	organization := bindOrganizationFlags(fs)
	instance := fs.String("instance", "", "Zendesk instance URL override")
	format := fs.String("format", "compact", "Output format: json or compact")
	grepType := fs.String("type", "", "Search result type filter: ticket, user, organization")
	limit := fs.Int("limit", 10, "Result page size")
	page := fs.Int("page", 1, "Search result page")
	_ = fs.Parse(reorderFlagArgs(args))

	if fs.NArg() < 1 {
		fatalErr(fmt.Errorf("grep requires a query string"))
	}

	resolver := config.NewResolver(
		config.Runtime{
			GOOS:          runtime.GOOS,
			UserConfigDir: os.UserConfigDir,
			Getenv:        os.Getenv,
		},
		config.NewKeychainStore(keyring.Get, keyring.Set, keyring.Delete),
	)

	client, err := newZendeskClientFromResolver(resolver, config.Source(*source), *organization, *instance)
	if err != nil {
		fatalErr(err)
	}

	result, err := zendesk.NewQueryEngine(client).Grep(context.Background(), strings.Join(fs.Args(), " "), zendesk.GrepOptions{
		Type:  *grepType,
		Limit: *limit,
		Page:  *page,
	})
	if err != nil {
		fatalErr(err)
	}

	if err := writeResults(*format, []zendesk.Result{result}); err != nil {
		fatalErr(err)
	}
}

func runAttachment(args []string) {
	if len(args) < 1 {
		usage()
		os.Exit(2)
	}

	switch args[0] {
	case "download":
		runAttachmentDownload(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown attachment command: %s\n\n", args[0])
		usage()
		os.Exit(2)
	}
}

func runAttachmentDownload(args []string) {
	fs := flag.NewFlagSet("attachment download", flag.ExitOnError)
	source := fs.String("source", string(config.SourceAuto), "Token source: auto, keychain, env_or_file")
	organization := bindOrganizationFlags(fs)
	instance := fs.String("instance", "", "Zendesk instance URL override")
	destination := fs.String("destination", "", "Destination file path or directory")
	output := fs.String("output", "", "Output file path (compat alias; prefer --destination)")
	dir := fs.String("dir", ".", "Output directory when --output is omitted (compat alias; prefer --destination DIR)")
	force := fs.Bool("force", false, "Overwrite existing file")
	_ = fs.Parse(reorderFlagArgs(args))

	if fs.NArg() != 1 {
		fatalErr(fmt.Errorf("attachment download requires exactly one attachment id"))
	}

	resolver := config.NewResolver(
		config.Runtime{
			GOOS:          runtime.GOOS,
			UserConfigDir: os.UserConfigDir,
			Getenv:        os.Getenv,
		},
		config.NewKeychainStore(keyring.Get, keyring.Set, keyring.Delete),
	)

	client, err := newZendeskClientFromResolver(resolver, config.Source(*source), *organization, *instance)
	if err != nil {
		fatalErr(err)
	}

	downloaded, err := client.DownloadAttachment(context.Background(), fs.Arg(0))
	if err != nil {
		fatalErr(err)
	}

	targetPath := resolveDestinationPath(downloaded.FileName, *destination, *output, *dir)

	if !*force {
		if _, err := os.Stat(targetPath); err == nil {
			fatalErr(fmt.Errorf("output file already exists: %s", targetPath))
		}
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		fatalErr(fmt.Errorf("create output directory: %w", err))
	}
	if err := os.WriteFile(targetPath, downloaded.Body, 0o600); err != nil {
		fatalErr(fmt.Errorf("write attachment file: %w", err))
	}

	out := struct {
		AttachmentID string `json:"attachment_id"`
		FileName     string `json:"file_name"`
		ContentType  string `json:"content_type,omitempty"`
		BytesWritten int    `json:"bytes_written"`
		Destination  string `json:"destination"`
	}{
		AttachmentID: fs.Arg(0),
		FileName:     downloaded.FileName,
		ContentType:  downloaded.ContentType,
		BytesWritten: len(downloaded.Body),
		Destination:  targetPath,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fatalErr(err)
	}
}

func resolveDestinationPath(fileName, destination, output, dir string) string {
	targetPath := strings.TrimSpace(destination)
	if targetPath != "" {
		if info, err := os.Stat(targetPath); err == nil && info.IsDir() {
			return filepath.Join(targetPath, fileName)
		}
		if hasPathSeparatorSuffix(targetPath) {
			return filepath.Join(targetPath, fileName)
		}
		return targetPath
	}

	targetPath = strings.TrimSpace(output)
	if targetPath != "" {
		return targetPath
	}

	baseDir := strings.TrimSpace(dir)
	if baseDir == "" {
		baseDir = "."
	}
	return filepath.Join(baseDir, fileName)
}

func hasPathSeparatorSuffix(path string) bool {
	return strings.HasSuffix(path, "/") || strings.HasSuffix(path, "\\")
}

func runAuthWriteConfig(args []string, resolver *config.Resolver) {
	fs := flag.NewFlagSet("auth write-config", flag.ExitOnError)
	organization := bindOrganizationFlags(fs)
	email := fs.String("email", "", "Zendesk user email for API token auth")
	token := bindTokenFlags(fs)
	pathOverride := fs.String("path", "", "Override auth.json path")
	_ = fs.Parse(args)

	path := *pathOverride
	if path == "" {
		var err error
		path, err = resolver.AuthConfigPath()
		if err != nil {
			fatalErr(err)
		}
	}

	org := config.NormalizeSuffix(*organization)
	if org == "" {
		fatalErr(fmt.Errorf("--organization is required"))
	}
	if strings.TrimSpace(*email) == "" {
		fatalErr(fmt.Errorf("--email is required"))
	}
	if strings.TrimSpace(*token) == "" {
		fatalErr(fmt.Errorf("--token is required"))
	}

	cfg := config.FileConfig{
		Profiles: map[string]config.FileProfile{
			org: {
				Email:    strings.TrimSpace(*email),
				APIToken: strings.TrimSpace(*token),
				AuthType: config.AuthTypeAPIToken,
			},
		},
	}
	if err := config.WriteFileConfig(path, cfg); err != nil {
		fatalErr(err)
	}

	fmt.Println(path)
}

func runAuthResolve(args []string, resolver *config.Resolver) {
	fs := flag.NewFlagSet("auth resolve", flag.ExitOnError)
	source := fs.String("source", string(config.SourceAuto), "Token source: auto, keychain, env_or_file")
	organization := bindOrganizationFlags(fs)
	instance := fs.String("instance", "", "Zendesk instance URL (required for keychain mode)")
	_ = fs.Parse(args)

	resolved, err := resolver.ResolveToken(config.ResolveOptions{
		Source:      config.Source(*source),
		InstanceURL: *instance,
		OrgSuffix:   *organization,
	})
	if err != nil {
		fatalErr(err)
	}

	out := struct {
		Source          string `json:"source"`
		ResolvedFrom    string `json:"resolved_from"`
		AuthType        string `json:"auth_type,omitempty"`
		Email           string `json:"email,omitempty"`
		ConfigPath      string `json:"config_path,omitempty"`
		APITokenPresent bool   `json:"api_token_present"`
	}{
		Source:          string(resolved.Source),
		ResolvedFrom:    resolved.ResolvedFrom,
		AuthType:        string(resolved.AuthType),
		Email:           resolved.Email,
		ConfigPath:      resolved.ConfigPath,
		APITokenPresent: resolved.Token != "",
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fatalErr(err)
	}
}

func runAuthSetAccess(args []string, resolver *config.Resolver) {
	fs := flag.NewFlagSet("auth set-access", flag.ExitOnError)
	source := fs.String("source", string(config.SourceAuto), "Token source: auto, keychain, env_or_file")
	organization := bindOrganizationFlags(fs)
	email := fs.String("email", "", "Zendesk user email for API token auth")
	token := bindTokenFlags(fs)
	_ = fs.Parse(args)

	result, err := resolver.SetAccess(config.SetAccessOptions{
		Source:    config.Source(*source),
		OrgSuffix: *organization,
		Email:     *email,
		Token:     *token,
	})
	if err != nil {
		fatalErr(err)
	}

	out := struct {
		Source       string `json:"source"`
		StoredIn     string `json:"stored_in"`
		Organization string `json:"organization"`
		AuthType     string `json:"auth_type"`
		Email        string `json:"email"`
		InstanceURL  string `json:"instance_url,omitempty"`
		ConfigPath   string `json:"config_path,omitempty"`
		SectionName  string `json:"section_name,omitempty"`
	}{
		Source:       string(result.Source),
		StoredIn:     result.StoredIn,
		Organization: result.OrgSuffix,
		AuthType:     string(result.AuthType),
		Email:        result.Email,
		InstanceURL:  result.InstanceURL,
		ConfigPath:   result.ConfigPath,
		SectionName:  result.SectionName,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fatalErr(err)
	}
}

func runAuthWhoAmI(args []string, resolver *config.Resolver) {
	fs := flag.NewFlagSet("auth whoami", flag.ExitOnError)
	source := fs.String("source", string(config.SourceAuto), "Token source: auto, keychain, env_or_file")
	organization := bindOrganizationFlags(fs)
	instance := fs.String("instance", "", "Zendesk instance URL")
	check := fs.Bool("check", true, "Perform a live Zendesk auth check with the stored credentials")
	_ = fs.Parse(args)

	status, err := resolver.InspectAccess(config.ResolveOptions{
		Source:      config.Source(*source),
		OrgSuffix:   *organization,
		InstanceURL: *instance,
	})
	if err != nil {
		fatalErr(err)
	}

	liveCheck := struct {
		Attempted  bool   `json:"attempted"`
		OK         bool   `json:"ok"`
		HTTPStatus int    `json:"http_status,omitempty"`
		UserID     int64  `json:"user_id,omitempty"`
		Name       string `json:"name,omitempty"`
		Email      string `json:"email,omitempty"`
		Role       string `json:"role,omitempty"`
		Active     bool   `json:"active,omitempty"`
		Suspended  bool   `json:"suspended,omitempty"`
		Error      string `json:"error,omitempty"`
	}{}

	if *check {
		liveCheck.Attempted = true

		resolved, resolveErr := resolver.ResolveToken(config.ResolveOptions{
			Source:      config.Source(*source),
			OrgSuffix:   *organization,
			InstanceURL: *instance,
		})
		if resolveErr != nil {
			liveCheck.Error = resolveErr.Error()
		} else {
			instanceURL := strings.TrimSpace(*instance)
			if instanceURL == "" {
				instanceURL = status.InstanceURL
			}
			if instanceURL == "" {
				instanceURL = config.InstanceURLFromSuffix(*organization)
			}

			checkResult, checkErr := zendesk.NewClient(&http.Client{}).CheckAuth(context.Background(), instanceURL, resolved)
			if checkErr != nil {
				liveCheck.Error = checkErr.Error()
				if httpErr, ok := checkErr.(*zendesk.HTTPError); ok {
					liveCheck.HTTPStatus = httpErr.StatusCode
				}
			} else {
				liveCheck.OK = true
				liveCheck.HTTPStatus = checkResult.HTTPStatus
				liveCheck.UserID = checkResult.UserID
				liveCheck.Name = checkResult.Name
				liveCheck.Email = checkResult.Email
				liveCheck.Role = checkResult.Role
				liveCheck.Active = checkResult.Active
				liveCheck.Suspended = checkResult.Suspended
			}
		}
	}

	out := struct {
		Source            string      `json:"source"`
		StoredIn          string      `json:"stored_in"`
		ResolvedFrom      string      `json:"resolved_from,omitempty"`
		Organization      string      `json:"organization,omitempty"`
		AuthType          string      `json:"auth_type,omitempty"`
		Email             string      `json:"email,omitempty"`
		InstanceURL       string      `json:"instance_url,omitempty"`
		ConfigPath        string      `json:"config_path,omitempty"`
		SectionName       string      `json:"section_name,omitempty"`
		APITokenPresent   bool        `json:"api_token_present"`
		AvailableProfiles []string    `json:"available_profiles,omitempty"`
		LiveCheck         interface{} `json:"live_check,omitempty"`
	}{
		Source:            string(status.Source),
		StoredIn:          status.StoredIn,
		ResolvedFrom:      status.ResolvedFrom,
		Organization:      status.OrgSuffix,
		AuthType:          string(status.AuthType),
		Email:             status.Email,
		InstanceURL:       status.InstanceURL,
		ConfigPath:        status.ConfigPath,
		SectionName:       status.SectionName,
		APITokenPresent:   status.AccessTokenPresent,
		AvailableProfiles: status.AvailableProfiles,
		LiveCheck:         liveCheck,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fatalErr(err)
	}
}

func runAuthClearAccess(args []string, resolver *config.Resolver) {
	fs := flag.NewFlagSet("auth clear-access", flag.ExitOnError)
	source := fs.String("source", string(config.SourceAuto), "Token source: auto, keychain, env_or_file")
	organization := bindOrganizationFlags(fs)
	instance := fs.String("instance", "", "Zendesk instance URL")
	_ = fs.Parse(args)

	result, err := resolver.ClearAccess(config.ResolveOptions{
		Source:      config.Source(*source),
		OrgSuffix:   *organization,
		InstanceURL: *instance,
	})
	if err != nil {
		fatalErr(err)
	}

	out := struct {
		Source       string `json:"source"`
		StoredIn     string `json:"stored_in"`
		Organization string `json:"organization,omitempty"`
		InstanceURL  string `json:"instance_url,omitempty"`
		ConfigPath   string `json:"config_path,omitempty"`
		SectionName  string `json:"section_name,omitempty"`
		Removed      bool   `json:"removed"`
	}{
		Source:       string(result.Source),
		StoredIn:     result.StoredIn,
		Organization: result.OrgSuffix,
		InstanceURL:  result.InstanceURL,
		ConfigPath:   result.ConfigPath,
		SectionName:  result.SectionName,
		Removed:      result.Removed,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fatalErr(err)
	}
}

func newZendeskClientFromResolver(resolver *config.Resolver, source config.Source, organization, instance string) (*zendesk.Client, error) {
	resolved, err := resolver.ResolveToken(config.ResolveOptions{
		Source:      source,
		OrgSuffix:   organization,
		InstanceURL: instance,
	})
	if err != nil {
		return nil, err
	}

	instanceURL := strings.TrimSpace(instance)
	if instanceURL == "" {
		instanceURL = config.InstanceURLFromSuffix(organization)
	}
	if instanceURL == "" {
		return nil, config.ErrInstanceURLRequired
	}

	return zendesk.NewAuthenticatedClient(instanceURL, resolved, &http.Client{})
}

func writeResults(format string, results []zendesk.Result) error {
	switch normalizeFormat(format) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(zendesk.JSONValue(results))
	case "compact":
		text, err := zendesk.RenderCompact(results)
		if err != nil {
			return err
		}
		if text == "" {
			return nil
		}
		fmt.Println(text)
		return nil
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func normalizeFormat(format string) string {
	value := strings.TrimSpace(strings.ToLower(format))
	switch value {
	case "", "json":
		return "json"
	case "compact", "llm":
		return "compact"
	default:
		return value
	}
}

func reorderFlagArgs(args []string) []string {
	if len(args) < 2 {
		return args
	}

	reordered := make([]string, 0, len(args))
	positionals := make([]string, 0, len(args))

	for idx := 0; idx < len(args); idx++ {
		arg := args[idx]
		if strings.HasPrefix(arg, "-") {
			reordered = append(reordered, arg)
			if !strings.Contains(arg, "=") && idx+1 < len(args) && !strings.HasPrefix(args[idx+1], "-") {
				reordered = append(reordered, args[idx+1])
				idx++
			}
			continue
		}
		positionals = append(positionals, arg)
	}

	return append(reordered, positionals...)
}

func fatalErr(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func bindOrganizationFlags(fs *flag.FlagSet) *string {
	var organization string
	fs.StringVar(&organization, "organization", "", "Zendesk organization, for example acme")
	fs.StringVar(&organization, "suffix", "", "Deprecated alias for --organization")
	return &organization
}

func bindTokenFlags(fs *flag.FlagSet) *string {
	var token string
	fs.StringVar(&token, "token", "", "Zendesk API token")
	fs.StringVar(&token, "access-token", "", "Deprecated alias for --token")
	return &token
}
