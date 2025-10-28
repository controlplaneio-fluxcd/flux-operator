// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	authfactory "github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth/factory"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/config"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/prompter"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/toolbox"
)

var (
	VERSION = "0.0.0-dev.0"

	transportOptions = strings.Join([]string{
		config.TransportSTDIO,
		config.TransportHTTP,
		config.TransportSSE,
	}, ", ")

	mcpImpl = &mcp.Implementation{
		Name:    "flux-operator-mcp",
		Version: VERSION,
	}
)

var rootCmd = &cobra.Command{
	Use:               "flux-operator-mcp",
	Version:           VERSION,
	SilenceUsage:      true,
	SilenceErrors:     true,
	DisableAutoGenTag: true,
	Long: `Model Context Protocol Server for interacting with Flux Operator.
⚠️ Please note that this MCP server is in preview and under development.
While we try our best not to introduce breaking changes, they may occur when
we adapt to new features and/or find better ways to facilitate what it does.`,
}

type rootFlags struct {
	timeout     time.Duration
	maskSecrets bool
	readOnly    bool
	transport   string
	port        int
	configFile  string
}

var (
	rootArgs = rootFlags{
		timeout: time.Minute,
	}
	kubeconfigArgs = genericclioptions.NewConfigFlags(false)
)

func init() {
	rootCmd.PersistentFlags().DurationVar(&rootArgs.timeout, "timeout", rootArgs.timeout,
		"The length of time to wait before giving up on the current operation.")
	rootCmd.PersistentFlags().BoolVar(&rootArgs.maskSecrets, "mask-secrets", true,
		"Mask secrets in the MCP server output")
	rootCmd.PersistentFlags().BoolVar(&rootArgs.readOnly, "read-only", false,
		"Run the MCP server in read-only mode, disabling write and delete operations.")
	rootCmd.PersistentFlags().StringVar(&rootArgs.transport, "transport", config.TransportSTDIO,
		fmt.Sprintf("The transport protocol to use for the MCP server. Options: [%s].", transportOptions))
	rootCmd.PersistentFlags().IntVar(&rootArgs.port, "port", 8080,
		"The port to use for the MCP server. This is only used when the transport is not 'stdio'.")
	rootCmd.PersistentFlags().StringVar(&rootArgs.configFile, "config", "",
		"The path to the configuration file.")
	addKubeConfigFlags(rootCmd)
	rootCmd.SetOut(os.Stdout)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(debugCmd)
	debugCmd.AddCommand(debugScopesCmd)
}

func main() {
	ctrllog.SetLogger(logr.New(ctrllog.NullLogSink{}))

	if err := rootCmd.Execute(); err != nil {
		rootCmd.PrintErrf("✗ %v\n", err)
		os.Exit(1)
	}
}

// addKubeConfigFlags maps the kubectl config flags to the given persistent flags.
// The default namespace is set to the value found in current kubeconfig context.
func addKubeConfigFlags(cmd *cobra.Command) {
	namespace := "default"
	// Try to read the default namespace from the current context.
	if ns, _, err := kubeconfigArgs.ToRawKubeConfigLoader().Namespace(); err == nil {
		namespace = ns
	}
	kubeconfigArgs.Namespace = &namespace

	cmd.PersistentFlags().StringVar(kubeconfigArgs.KubeConfig, "kubeconfig", getCurrentKubeconfigPath(),
		"Path to the kubeconfig file.")
	cmd.PersistentFlags().StringVar(kubeconfigArgs.Context, "kube-context", "",
		"The name of the kubeconfig context to use.")
	cmd.PersistentFlags().StringVar(kubeconfigArgs.Impersonate, "kube-as", "",
		"Username to impersonate for the operation. User could be a regular user or a service account in a namespace.")
	cmd.PersistentFlags().StringArrayVar(kubeconfigArgs.ImpersonateGroup, "kube-as-group", nil,
		"Group to impersonate for the operation, this flag can be repeated to specify multiple groups.")
	cmd.PersistentFlags().StringVar(kubeconfigArgs.ImpersonateUID, "kube-as-uid", "",
		"UID to impersonate for the operation.")
	cmd.PersistentFlags().StringVar(kubeconfigArgs.BearerToken, "kube-token", "",
		"Bearer token for authentication to the API server.")
	cmd.PersistentFlags().StringVar(kubeconfigArgs.APIServer, "kube-server", "",
		"The address and port of the Kubernetes API server.")
	cmd.PersistentFlags().StringVar(kubeconfigArgs.TLSServerName, "kube-tls-server-name", "",
		"Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used.")
	cmd.PersistentFlags().StringVar(kubeconfigArgs.CertFile, "kube-client-certificate", "",
		"Path to a client certificate file for TLS.")
	cmd.PersistentFlags().StringVar(kubeconfigArgs.KeyFile, "kube-client-key", "",
		"Path to a client key file for TLS.")
	cmd.PersistentFlags().StringVar(kubeconfigArgs.CAFile, "kube-certificate-authority", "",
		"Path to a cert file for the certificate authority.")
	cmd.PersistentFlags().BoolVar(kubeconfigArgs.Insecure, "kube-insecure-skip-tls-verify", false,
		"if true, the Kubernetes API server's certificate will not be checked for validity. This will make your HTTPS connections insecure.")
	cmd.PersistentFlags().StringVarP(kubeconfigArgs.Namespace, "namespace", "n",
		*kubeconfigArgs.Namespace, "The the namespace scope for the operation.")
}

func getCurrentKubeconfigPath() string {
	defaultPath := ""

	kubeConfig := os.Getenv("KUBECONFIG")
	if kubeConfig == "" {
		return defaultPath
	}

	paths := filepath.SplitList(kubeConfig)
	if len(paths) == 1 {
		return paths[0]
	}

	var currentContext string
	for _, path := range paths {
		conf, err := clientcmd.LoadFromFile(path)
		if err != nil {
			continue
		}
		if currentContext == "" {
			currentContext = conf.CurrentContext
		}
		_, ok := conf.Contexts[currentContext]
		if ok {
			return path
		}
	}
	return defaultPath
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server",
	RunE:  serveCmdRun,
}

func serveCmdRun(cmd *cobra.Command, args []string) error {
	var conf config.Config
	if rootArgs.configFile != "" {
		b, err := os.ReadFile(rootArgs.configFile)
		if err != nil {
			return fmt.Errorf("failed to read config file '%s': %w", rootArgs.configFile, err)
		}
		if err := yaml.Unmarshal(b, &conf); err != nil {
			return fmt.Errorf("failed to unmarshal config file '%s' as YAML: %w", rootArgs.configFile, err)
		}
		if conf.GroupVersionKind() != config.GroupVersion.WithKind(config.ConfigKind) {
			return fmt.Errorf("invalid config file '%s': expected apiVersion '%s' and kind '%s', got '%s' and '%s'",
				rootArgs.configFile, config.GroupVersion.String(), config.ConfigKind, conf.APIVersion, conf.Kind)
		}
		if conf.Spec.Transport == config.TransportSSE {
			return fmt.Errorf("the '%s' transport is not supported in the Config API", config.TransportSSE)
		}
		if conf.Spec.Transport == "" {
			return fmt.Errorf("the 'transport' field is required in the Config API")
		}
	}

	transport := conf.Spec.Transport
	if transport == "" { // If --config is not set, fallback to the CLI flag.
		transport = rootArgs.transport
	}

	readOnly := conf.Spec.ReadOnly
	if !readOnly { // If --config is not set, fallback to the CLI flag.
		readOnly = rootArgs.readOnly
	}

	inCluster := os.Getenv("KUBERNETES_SERVICE_HOST") != ""
	if inCluster {
		log.Printf("Starting the MCP server in-cluster with read-only mode set to %t", readOnly)
	}

	if os.Getenv("KUBECONFIG") == "" && !inCluster {
		return errors.New("KUBECONFIG environment variable is not set")
	}

	// Create the MCP server
	mcpServer := mcp.NewServer(mcpImpl, &mcp.ServerOptions{
		HasTools:   true,
		HasPrompts: true,
	})

	// Add authentication middleware if configured
	if conf.Spec.Authentication != nil {
		if transport != config.TransportHTTP {
			return fmt.Errorf("authentication is only supported with the '%s' transport", config.TransportHTTP)
		}

		authFunc, err := authfactory.New(*conf.Spec.Authentication)
		if err != nil {
			return fmt.Errorf("failed to create authentication layer: %w", err)
		}

		// Add receiving middleware for authentication
		mcpServer.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
			return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				header := req.GetExtra().Header
				switch method {
				case "tools/call":
					ctx, err := authFunc(ctx, header)
					if err != nil {
						res, _, _ := toolbox.NewToolResultError(err.Error())
						return res, nil
					}
					return next(ctx, method, req)
				case "tools/list":
					res, err := next(ctx, method, req)
					if err != nil {
						return res, err
					}
					authCtx, err := authFunc(ctx, header)
					if err != nil {
						authCtx = nil
					}
					toolbox.AddScopesAndFilter(authCtx, res.(*mcp.ListToolsResult), readOnly)
					return res, nil
				default:
					return next(ctx, method, req)
				}
			}
		})
	}

	// Register tools and prompts
	tm := toolbox.NewManager(kubeconfigArgs, rootArgs.timeout, rootArgs.maskSecrets, readOnly)
	tm.RegisterTools(mcpServer, inCluster)

	pm := prompter.NewManager()
	pm.RegisterPrompts(mcpServer)

	// Start server based on transport type
	var mcpHandler http.Handler
	ctx := ctrl.SetupSignalHandler()

	switch transport {
	case config.TransportSTDIO:
		if err := mcpServer.Run(ctx, &mcp.StdioTransport{}); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("failed to run MCP server over stdio: %w", err)
		}
		return nil
	case config.TransportHTTP:
		handler := mcp.NewStreamableHTTPHandler(
			func(*http.Request) *mcp.Server { return mcpServer },
			&mcp.StreamableHTTPOptions{},
		)
		mux := http.NewServeMux()
		mux.Handle("/mcp", handler)
		mcpHandler = mux
	case config.TransportSSE:
		log.Printf("⚠️ The '%s' transport is still supported but is now considered legacy. Please switch to the '%s' transport.",
			config.TransportSSE, config.TransportHTTP)
		handler := mcp.NewSSEHandler(
			func(*http.Request) *mcp.Server { return mcpServer },
			&mcp.SSEOptions{},
		)
		mux := http.NewServeMux()
		mux.Handle("/sse", handler)
		mcpHandler = mux
	default:
		return fmt.Errorf("unknown transport: '%s'", transport)
	}

	// If the code reaches here, we have an HTTP server to start.
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", rootArgs.port),
		Handler: mcpHandler,
	}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()
	<-ctx.Done()
	log.Println("Shutting down HTTP server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Failed to shutdown HTTP server: %v", err)
	}
	log.Println("HTTP server stopped.")
	return nil
}

var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Debug the MCP server",
}

var debugScopesCmd = &cobra.Command{
	Use:   "scopes <Flux MCP URL>",
	Short: "Debug the MCP server scopes",
	Args:  cobra.ExactArgs(1),
	RunE:  debugScopesCmdRun,
}

func debugScopesCmdRun(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), rootArgs.timeout)
	defer cancel()

	endpoint, err := url.Parse(args[0])
	if err != nil {
		return fmt.Errorf("failed to parse MCP server endpoint: %w", err)
	}
	endpoint.Path = "/mcp"

	client := mcp.NewClient(mcpImpl, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint.String()}, nil)
	if err != nil {
		return fmt.Errorf("failed to create MCP client for %s: %w", endpoint, err)
	}
	defer cs.Close()

	resp, err := cs.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return fmt.Errorf("failed to list MCP tools: %w", err)
	}

	scopes, ok := resp.Meta["scopes"]
	if !ok {
		return errors.New("the MCP server did not return the available scopes")
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	if err := enc.Encode(scopes); err != nil {
		return fmt.Errorf("failed to marshal scopes to JSON: %w", err)
	}

	return nil
}
