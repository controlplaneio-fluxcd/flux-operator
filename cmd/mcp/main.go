// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/prompter"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/toolbox"
)

var (
	VERSION = "0.0.0-dev.0"

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
	rootCmd.PersistentFlags().StringVar(&rootArgs.transport, "transport", "stdio",
		"The transport protocol to use for the MCP server. Options: [stdio, http, sse].")
	rootCmd.PersistentFlags().IntVar(&rootArgs.port, "port", 8080,
		"The port to use for the MCP server. This is only used when the transport is not 'stdio'.")
	addKubeConfigFlags(rootCmd)
	rootCmd.SetOut(os.Stdout)
	rootCmd.AddCommand(serveCmd)
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
	inCluster := os.Getenv("KUBERNETES_SERVICE_HOST") != ""
	if inCluster {
		log.Printf("Starting the MCP server in-cluster with read-only mode set to %t", rootArgs.readOnly)
	}

	if os.Getenv("KUBECONFIG") == "" && !inCluster {
		return errors.New("KUBECONFIG environment variable is not set")
	}

	// Create the MCP server
	mcpServer := mcp.NewServer(mcpImpl, &mcp.ServerOptions{
		HasTools:   true,
		HasPrompts: true,
	})

	// Register tools and prompts
	kubeClient := k8s.NewClientFactory(kubeconfigArgs)
	tm := toolbox.NewManager(kubeClient, rootArgs.timeout, rootArgs.maskSecrets, rootArgs.readOnly, nil)
	tm.RegisterTools(mcpServer, inCluster)

	pm := prompter.NewManager()
	pm.RegisterPrompts(mcpServer)

	// Start server based on transport type
	var mcpHandler http.Handler
	ctx := ctrl.SetupSignalHandler()

	switch rootArgs.transport {
	case "stdio":
		if err := mcpServer.Run(ctx, &mcp.StdioTransport{}); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("failed to run MCP server over stdio: %w", err)
		}
		return nil
	case "http":
		handler := mcp.NewStreamableHTTPHandler(
			func(*http.Request) *mcp.Server { return mcpServer },
			&mcp.StreamableHTTPOptions{},
		)
		mux := http.NewServeMux()
		mux.Handle("/mcp", handler)
		mcpHandler = mux
	case "sse":
		log.Println("⚠️ The 'sse' transport is still supported but is now considered legacy. Please switch to the 'http' transport.")
		handler := mcp.NewSSEHandler(
			func(*http.Request) *mcp.Server { return mcpServer },
			&mcp.SSEOptions{},
		)
		mux := http.NewServeMux()
		mux.Handle("/sse", handler)
		mcpHandler = mux
	default:
		return fmt.Errorf("unknown transport: '%s'", rootArgs.transport)
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
