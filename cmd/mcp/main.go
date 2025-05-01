// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/prompter"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/toolbox"
)

var (
	VERSION = "0.0.0-dev.0"
)

var rootCmd = &cobra.Command{
	Use:               "flux-operator-mcp",
	Version:           VERSION,
	SilenceUsage:      true,
	SilenceErrors:     true,
	DisableAutoGenTag: true,
	Long: `Model Context Protocol Server for interacting with Flux Operator.
⚠️ Please note that this MCP server is in preview and under development.
While we try our best to not introduce breaking changes, they may occur when
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
		"The transport protocol to use for the MCP server. Options: [stdio, sse].")
	rootCmd.PersistentFlags().IntVar(&rootArgs.port, "port", 8080,
		"The port to use for the MCP server. This is only used when the transport is set to 'sse'.")
	addKubeConfigFlags(rootCmd)
	rootCmd.SetOut(os.Stdout)
	rootCmd.AddCommand(serveCmd)
}

func main() {
	log.SetFlags(0)
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
	kubeConfig := os.Getenv("KUBECONFIG")
	if kubeConfig == "" {
		panic("KUBECONFIG environment variable not set")
	}

	paths := filepath.SplitList(kubeConfig)
	if len(paths) == 1 {
		return paths[0]
	}

	var currentContext string
	for _, path := range paths {
		config, err := clientcmd.LoadFromFile(path)
		if err != nil {
			continue
		}
		if currentContext == "" {
			currentContext = config.CurrentContext
		}
		_, ok := config.Contexts[currentContext]
		if ok {
			return path
		}
	}
	return kubeConfig
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server in stdio or sse mode",
	RunE:  serveCmdRun,
}

func serveCmdRun(cmd *cobra.Command, args []string) error {
	mcpServer := server.NewMCPServer(
		"flux-operator-mcp",
		VERSION,
		server.WithResourceCapabilities(true, true),
		server.WithToolCapabilities(true),
		server.WithPromptCapabilities(true),
	)

	tm := toolbox.NewManager(kubeconfigArgs, rootArgs.timeout, rootArgs.maskSecrets)
	tm.RegisterTools(mcpServer, rootArgs.readOnly)

	pm := prompter.NewManager()
	pm.RegisterPrompts(mcpServer)

	if rootArgs.transport == "sse" {
		sseServer := server.NewSSEServer(mcpServer)
		if err := sseServer.Start(fmt.Sprintf(":%d", rootArgs.port)); err != nil {
			return err
		}
	} else {
		if err := server.ServeStdio(mcpServer); err != nil {
			return err
		}
	}

	return nil
}
