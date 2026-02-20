// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	ghauth "github.com/cli/go-gh/v2/pkg/auth"
	"github.com/google/go-github/v81/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"sigs.k8s.io/yaml"

	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/version"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// fluxControllerBaseURL is the base URL for fetching controller CRD files.
// It can be overridden in tests to point to a mock server.
var fluxControllerBaseURL = "https://github.com/fluxcd"

var patchInstanceCmd = &cobra.Command{
	Use:   "instance",
	Short: "Generate kustomize patches for upgrading Flux controllers in a FluxInstance",
	Long: `The patch instance command automates the generation of kustomize patches needed
to upgrade Flux controllers to a newer version. It modifies the FluxInstance YAML
manifest file in-place, appending patches to .spec.kustomize.patches.

The command performs the following steps:
  1. Reads the FluxInstance YAML manifest and resolves the current Flux minor version
     from .spec.distribution.version (supports exact versions like '2.7.0' and semver
     constraints like '2.x' resolved via GitHub tags).
  2. Resolves the target version from the --version flag ('main', 'v2.8', '8', etc.).
  3. For each controller listed in --components (or .spec.components if not specified):
     a. Fetches the CRD schemas for both the current and target controller versions
        from the fluxcd GitHub repositories.
     b. Computes a JSON patch (RFC 6902) for each CRD that changed between versions.
        This includes schema additions, field removals, enum updates, and validation
        rule changes. CRDs with no differences are skipped.
     c. Generates a Deployment image patch pointing to the target controller version.
  4. Appends all generated patches to .spec.kustomize.patches in the manifest file,
     preserving the original YAML formatting (field order, quoting, indentation).
     If the kustomize section or patches array does not exist, it is created.

The controller version mapping is derived from the fluxcd/pkg/version package, which
maps each Flux distribution minor version to the corresponding controller versions
(e.g. Flux v2.7 maps to source-controller v1.7, helm-controller v1.4, etc.).

When --version is set to 'main', the command targets the next unreleased version by
fetching CRDs from the main branch. Image tags are set to 'rc-<sha>' using the first
8 characters of each controller's main branch HEAD commit, with the registry fixed to
ghcr.io/fluxcd.

GitHub authentication is optional but recommended to avoid rate limits. The command
uses credentials from the GH_TOKEN or GITHUB_TOKEN environment variables, or from
credentials stored by 'gh auth login'.`,
	Example: `  # Generate patches for upgrading to the latest development version (main branch)
  flux-operator patch instance -f instance.yaml

  # Generate patches for upgrading from the current version to Flux v2.8
  flux-operator patch instance -f instance.yaml -v v2.8

  # Same as above using the short minor version form
  flux-operator patch instance -f instance.yaml -v 8

  # Override the container registry for image patches
  flux-operator patch instance -f instance.yaml -v 8 -r registry.example.com/fluxcd

  # Patch only specific controllers
  flux-operator patch instance -f instance.yaml -v 8 -c source-controller,helm-controller

  # Pipe input from stdin (patches are printed to stdout)
  cat instance.yaml | flux-operator patch instance -f - -v 8`,
	Args: cobra.NoArgs,
	RunE: patchInstanceCmdRun,
}

type patchInstanceFlags struct {
	filename   string
	version    string
	registry   string
	components []string
}

var patchInstanceArgs patchInstanceFlags

func init() {
	patchInstanceCmd.Flags().StringVarP(&patchInstanceArgs.filename, "filename", "f", "",
		"Path to the FluxInstance YAML manifest.")
	patchInstanceCmd.Flags().StringVarP(&patchInstanceArgs.version, "version", "v", "main",
		"Target Flux version. Accepts: 'main', 'v2.<minor>', or <minor> integer.")
	patchInstanceCmd.Flags().StringVarP(&patchInstanceArgs.registry, "registry", "r", "",
		"Override the container registry for image patches (defaults to .spec.distribution.registry).")
	patchInstanceCmd.Flags().StringSliceVarP(&patchInstanceArgs.components, "components", "c", nil,
		"Comma-separated list of controllers to patch (defaults to .spec.components).")

	patchCmd.AddCommand(patchInstanceCmd)
}

// patchTarget represents the resolved target version for patching.
type patchTarget struct {
	isMain    bool
	fluxMinor int
}

func patchInstanceCmdRun(cmd *cobra.Command, args []string) error {
	if patchInstanceArgs.filename == "" {
		return fmt.Errorf("--filename is required")
	}

	// Read the input file (supports stdin via "-").
	path := patchInstanceArgs.filename
	if patchInstanceArgs.filename == "-" {
		var err error
		path, err = saveReaderToFile(os.Stdin)
		if err != nil {
			return err
		}
		defer os.Remove(path)
	}

	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("invalid filename '%s', must point to an existing file", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	// Parse the FluxInstance to extract version and component information.
	var instance fluxcdv1.FluxInstance
	if err := yaml.Unmarshal(data, &instance); err != nil {
		return fmt.Errorf("parsing FluxInstance: %w", err)
	}
	setInstanceDefaults(&instance)
	if instance.Spec.Distribution.Version == "" {
		return fmt.Errorf(".spec.distribution.version is required")
	}

	// Resolve the current and target Flux minor versions.
	ctx, cancel := context.WithTimeout(cmd.Context(), rootArgs.timeout)
	defer cancel()

	rootCmd.PrintErrln(`◎`, "Resolving current version from", instance.Spec.Distribution.Version)
	fromMinor, err := resolveFromMinor(ctx, instance.Spec.Distribution.Version)
	if err != nil {
		return fmt.Errorf("resolving current version: %w", err)
	}
	target, err := resolveToTarget(patchInstanceArgs.version)
	if err != nil {
		return fmt.Errorf("resolving target version: %w", err)
	}
	if !target.isMain && target.fluxMinor <= fromMinor {
		return fmt.Errorf("target minor version %d must be greater than current minor version %d",
			target.fluxMinor, fromMinor)
	}
	targetLabel := "main"
	if !target.isMain {
		targetLabel = fmt.Sprintf("2.%d", target.fluxMinor)
	}
	rootCmd.PrintErrln(`✔`, fmt.Sprintf("Resolved version range: 2.%d -> %s", fromMinor, targetLabel))

	// Determine which components to patch.
	components := instance.GetComponents()
	if len(patchInstanceArgs.components) > 0 {
		components = patchInstanceArgs.components
	}

	// Remove previously generated patches (image and CRD) for the selected components.
	if instance.Spec.Kustomize != nil {
		var removed int
		data, removed = removeGeneratedPatches(data, instance.Spec.Kustomize.Patches, components)
		if removed > 0 {
			rootCmd.PrintErrln(`✔`, fmt.Sprintf("Removed %d previously generated patches", removed))
		}
	}

	// Compute image and CRD patches for each component.
	registry := instance.Spec.Distribution.Registry
	if patchInstanceArgs.registry != "" {
		registry = patchInstanceArgs.registry
	}
	if token, _ := ghauth.TokenForHost("github.com"); token != "" {
		rootCmd.PrintErrln(`✔`, "Using GitHub credentials for API requests")
	} else {
		rootCmd.PrintErrln(`◎`, "No GitHub credentials found, API requests may be rate-limited")
		rootCmd.PrintErrln(`◎`, "Run 'gh auth login' or set GITHUB_TOKEN to authenticate")
	}
	rootCmd.PrintErrln(`◎`, fmt.Sprintf("Computing patches for %d components...", len(components)))
	patches, err := computeAllPatches(ctx, components,
		fromMinor, target, registry)
	if err != nil {
		return err
	}
	if len(patches) == 0 {
		rootCmd.PrintErrln(`✔`, "No patches needed")
		return nil
	}

	// Append patches to the YAML and write back.
	out, err := appendPatchesToYAML(data, patches)
	if err != nil {
		return fmt.Errorf("appending patches: %w", err)
	}
	if err := writeOutput(out, path, len(patches)); err != nil {
		return err
	}

	// Validate the patched instance by running 'build instance'.
	if patchInstanceArgs.filename != "-" {
		rootCmd.PrintErrln(`◎`, "Validating patched instance...")
		if err := validatePatchedInstance(path); err != nil {
			return err
		}
		rootCmd.PrintErrln(`✔`, "Validation passed")
	}
	return nil
}

// validatePatchedInstance runs 'build instance' on the patched file to
// verify the generated patches produce valid Flux manifests.
func validatePatchedInstance(path string) error {
	saved := buildInstanceArgs.filename
	defer func() { buildInstanceArgs.filename = saved }()
	buildInstanceArgs.filename = path

	// Suppress the build output by temporarily redirecting rootCmd.
	origOut := rootCmd.OutOrStdout()
	rootCmd.SetOut(io.Discard)
	defer rootCmd.SetOut(origOut)

	if err := buildInstanceCmdRun(rootCmd, nil); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return nil
}

// writeOutput writes the patched YAML to the original file or stdout.
func writeOutput(out []byte, path string, patchCount int) error {
	if patchInstanceArgs.filename == "-" {
		rootCmd.Print(string(out))
		return nil
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}
	rootCmd.PrintErrln(`✔`, fmt.Sprintf("Appended %d patches to %s", patchCount, path))
	return nil
}

// computeAllPatches generates image and CRD patches for all components.
// Image patches are returned first, followed by CRD patches.
func computeAllPatches(
	ctx context.Context,
	components []string,
	fromMinor int,
	target patchTarget,
	registry string,
) ([]kustomize.Patch, error) {
	var imagePatches, crdPatches []kustomize.Patch

	for _, component := range components {
		rootCmd.PrintErrln(`◎`, fmt.Sprintf("Processing %s...", component))

		// Image patch for this controller's Deployment.
		imgPatch, err := computeImagePatch(ctx, component, target, registry)
		if err != nil {
			return nil, fmt.Errorf("computing image patch for %s: %w", component, err)
		}
		if imgPatch != nil {
			imagePatches = append(imagePatches, *imgPatch)
		}

		// CRD patches for each CRD owned by this controller.
		crdKinds, ok := fluxcdv1.ControllerCRDKinds[component]
		if !ok {
			rootCmd.PrintErrf("warning: no CRD mapping for component %q, skipping CRD patches\n", component)
			continue
		}
		for _, kind := range crdKinds {
			patch, err := computeCRDPatchForKind(ctx, component, kind, fromMinor, target)
			if err != nil {
				continue
			}
			if patch != nil {
				crdPatches = append(crdPatches, *patch)
			}
		}
	}

	return append(imagePatches, crdPatches...), nil
}

// computeCRDPatchForKind resolves the CRD URLs for a given kind and
// computes the diff patch. Returns nil if the CRD is unchanged.
func computeCRDPatchForKind(
	ctx context.Context,
	controller string,
	kind string,
	fromMinor int,
	target patchTarget,
) (*kustomize.Patch, error) {
	gk, err := fluxcdv1.FluxGroupFor(kind)
	if err != nil {
		return nil, fmt.Errorf("resolving group for kind %q: %w", kind, err)
	}
	kindInfo, err := fluxcdv1.FindFluxKindInfo(kind)
	if err != nil {
		return nil, fmt.Errorf("resolving kind info for %q: %w", kind, err)
	}

	crdName := kindInfo.Plural + "." + gk.Group
	rootCmd.PrintErrln(`◎`, fmt.Sprintf("  Diffing CRD %s", crdName))

	sourceURL, targetURL, err := buildCRDURLs(controller, gk.Group, kindInfo.Plural, fromMinor, target)
	if err != nil {
		return nil, fmt.Errorf("building CRD URLs for %s: %w", kind, err)
	}

	patch, err := computeCRDPatch(ctx, sourceURL, targetURL, gk.Group, kindInfo.Plural)
	if err != nil {
		rootCmd.PrintErrf("warning: computing CRD patch for %s: %v\n", crdName, err)
		return nil, err
	}
	return patch, nil
}

// appendPatchesToYAML inserts kustomize patches into a YAML document
// using raw string processing to preserve the original formatting.
// It handles three cases:
//   - No kustomize: key exists → adds kustomize.patches at end of spec block
//   - kustomize: exists but no patches: → adds patches under kustomize
//   - kustomize.patches: exists → appends items after existing patches
func appendPatchesToYAML(data []byte, patches []kustomize.Patch) ([]byte, error) {
	if len(patches) == 0 {
		return data, nil
	}

	content := strings.TrimRight(string(data), "\n")
	lines := strings.Split(content, "\n")
	indent := detectIndentSize(lines)
	patchesKeyIndent := indent * 2

	// Marshal the patches array at zero indent.
	patchesYAML, err := yaml.Marshal(patches)
	if err != nil {
		return nil, fmt.Errorf("marshalling patches: %w", err)
	}
	fragment := strings.TrimRight(string(patchesYAML), "\n")

	specIdx := findTopLevelKey(lines, "spec")
	if specIdx < 0 {
		return nil, fmt.Errorf("'spec' key not found in YAML")
	}

	kustomizeIdx := findChildKey(lines, specIdx, indent, "kustomize")

	var insertIdx int
	var insertText string

	if kustomizeIdx < 0 {
		// No kustomize: section, add it at end of spec block.
		insertIdx = findBlockEnd(lines, specIdx, 0)
		insertText = strings.Repeat(" ", indent) + "kustomize:\n" +
			strings.Repeat(" ", patchesKeyIndent) + "patches:\n" +
			prefixIndent(fragment, patchesKeyIndent)
	} else {
		patchesIdx := findChildKey(lines, kustomizeIdx, patchesKeyIndent, "patches")
		if patchesIdx < 0 {
			// kustomize: exists but no patches: key.
			insertIdx = findBlockEnd(lines, kustomizeIdx, indent)
			insertText = strings.Repeat(" ", patchesKeyIndent) + "patches:\n" +
				prefixIndent(fragment, patchesKeyIndent)
		} else {
			// patches: exists, clear any inline value (e.g. "patches: []")
			// and append items after the existing array.
			patchesLine := lines[patchesIdx]
			trimmed := strings.TrimSpace(patchesLine)
			if trimmed != "patches:" {
				lines[patchesIdx] = strings.Repeat(" ", countLeadingSpaces(patchesLine)) + "patches:"
			}
			itemIndent := detectArrayItemIndent(lines, patchesIdx, patchesKeyIndent)
			insertIdx = findPatchesArrayEnd(lines, patchesIdx, itemIndent)
			insertText = prefixIndent(fragment, itemIndent)
		}
	}

	insertionLines := strings.Split(insertText, "\n")
	result := make([]string, 0, len(lines)+len(insertionLines))
	result = append(result, lines[:insertIdx]...)
	result = append(result, insertionLines...)
	result = append(result, lines[insertIdx:]...)

	return []byte(strings.Join(result, "\n") + "\n"), nil
}

// detectIndentSize returns the indentation unit used in the YAML file
// by looking at the first indented line.
func detectIndentSize(lines []string) int {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		spaces := countLeadingSpaces(line)
		if spaces > 0 {
			return spaces
		}
	}
	return 2
}

// countLeadingSpaces returns the number of leading space characters.
func countLeadingSpaces(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

// prefixIndent prepends a fixed number of spaces to each non-empty line.
func prefixIndent(s string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

// findTopLevelKey returns the line index of a top-level YAML key (indent 0).
func findTopLevelKey(lines []string, key string) int {
	prefix := key + ":"
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if countLeadingSpaces(line) == 0 &&
			(trimmed == prefix || strings.HasPrefix(trimmed, prefix+" ")) {
			return i
		}
	}
	return -1
}

// findChildKey finds a key at the expected indent level within a parent block.
func findChildKey(lines []string, parentIdx int, childIndent int, key string) int {
	prefix := key + ":"
	for i := parentIdx + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		spaces := countLeadingSpaces(line)
		if spaces < childIndent {
			break
		}
		if spaces == childIndent &&
			(trimmed == prefix || strings.HasPrefix(trimmed, prefix+" ")) {
			return i
		}
	}
	return -1
}

// findBlockEnd returns the index of the first line after startIdx
// that exits the block (indent <= parentIndent).
func findBlockEnd(lines []string, startIdx int, parentIndent int) int {
	for i := startIdx + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if countLeadingSpaces(line) <= parentIndent {
			return i
		}
	}
	return len(lines)
}

// findPatchesArrayEnd returns the index of the first line after patchesIdx
// that is no longer part of the patches array. Array items start with "- "
// at the patchesIndent level; deeper lines are item content.
// detectArrayItemIndent returns the indentation of the first array item
// after the patches: key. Falls back to the provided default if no items exist.
func detectArrayItemIndent(lines []string, patchesIdx int, fallback int) int {
	for i := patchesIdx + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			return countLeadingSpaces(line)
		}
		break
	}
	return fallback
}

func findPatchesArrayEnd(lines []string, patchesIdx int, patchesIndent int) int {
	for i := patchesIdx + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		spaces := countLeadingSpaces(line)
		if spaces < patchesIndent {
			return i
		}
		if spaces == patchesIndent && !strings.HasPrefix(trimmed, "- ") {
			return i
		}
	}
	return len(lines)
}

// isGeneratedPatchForComponents returns true if the patch was generated by this
// command for one of the given components. It checks Deployment image patches
// by controller name and CRD patches by CRD name.
func isGeneratedPatchForComponents(p kustomize.Patch, components []string) bool {
	if p.Target == nil {
		return false
	}
	if p.Target.Kind == "Deployment" &&
		strings.Contains(p.Patch, "/spec/template/spec/containers/0/image") {
		for _, c := range components {
			if p.Target.Name == c {
				return true
			}
		}
		return false
	}
	if p.Target.Kind == "CustomResourceDefinition" {
		for _, c := range components {
			kinds, ok := fluxcdv1.ControllerCRDKinds[c]
			if !ok {
				continue
			}
			for _, kind := range kinds {
				gk, err := fluxcdv1.FluxGroupFor(kind)
				if err != nil {
					continue
				}
				ki, err := fluxcdv1.FindFluxKindInfo(kind)
				if err != nil {
					continue
				}
				if p.Target.Name == ki.Plural+"."+gk.Group {
					return true
				}
			}
		}
		return false
	}
	return false
}

// removeGeneratedPatches removes previously generated image and CRD patches
// for the given components from the raw YAML, preserving all other content and formatting.
func removeGeneratedPatches(data []byte, patches []kustomize.Patch, components []string) ([]byte, int) {
	if len(patches) == 0 {
		return data, 0
	}

	content := strings.TrimRight(string(data), "\n")
	lines := strings.Split(content, "\n")
	indent := detectIndentSize(lines)
	patchesKeyIndent := indent * 2

	specIdx := findTopLevelKey(lines, "spec")
	if specIdx < 0 {
		return data, 0
	}
	kustomizeIdx := findChildKey(lines, specIdx, indent, "kustomize")
	if kustomizeIdx < 0 {
		return data, 0
	}
	patchesIdx := findChildKey(lines, kustomizeIdx, patchesKeyIndent, "patches")
	if patchesIdx < 0 {
		return data, 0
	}

	itemIndent := detectArrayItemIndent(lines, patchesIdx, patchesKeyIndent)
	arrayEnd := findPatchesArrayEnd(lines, patchesIdx, itemIndent)

	// Find all item start positions.
	var itemStarts []int
	for i := patchesIdx + 1; i < arrayEnd; i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if countLeadingSpaces(line) == itemIndent && strings.HasPrefix(trimmed, "- ") {
			itemStarts = append(itemStarts, i)
		}
	}

	if len(itemStarts) != len(patches) {
		return data, 0
	}

	// Determine line ranges for generated patches.
	type lineRange struct{ start, end int }
	var toRemove []lineRange

	for idx, patch := range patches {
		if !isGeneratedPatchForComponents(patch, components) {
			continue
		}
		start := itemStarts[idx]
		scanEnd := arrayEnd
		if idx+1 < len(itemStarts) {
			scanEnd = itemStarts[idx+1]
		}
		// Tighten range to last content line (preserve trailing comments).
		end := start + 1
		for j := start + 1; j < scanEnd; j++ {
			trimmed := strings.TrimSpace(lines[j])
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				end = j + 1
			}
		}
		toRemove = append(toRemove, lineRange{start, end})
	}

	if len(toRemove) == 0 {
		return data, 0
	}

	for i := len(toRemove) - 1; i >= 0; i-- {
		r := toRemove[i]
		lines = append(lines[:r.start], lines[r.end:]...)
	}

	return []byte(strings.Join(lines, "\n") + "\n"), len(toRemove)
}

// resolveFromMinor resolves the Flux minor version from a version expression.
// It handles both exact semver versions (e.g., "2.7.5") and semver constraints
// (e.g., "2.x", ">=2.7.0") by querying GitHub tags when needed.
func resolveFromMinor(ctx context.Context, versionExpr string) (int, error) {
	// Try parsing as an exact semver version first.
	v, err := semver.NewVersion(versionExpr)
	if err == nil {
		return int(v.Minor()), nil
	}

	// Treat as a semver constraint and resolve via GitHub tags.
	constraint, err := semver.NewConstraint(versionExpr)
	if err != nil {
		return 0, fmt.Errorf("invalid version expression %q: not a valid semver or constraint", versionExpr)
	}

	ghClient := newGitHubClient(ctx)
	tags, err := listFlux2Tags(ctx, ghClient)
	if err != nil {
		return 0, fmt.Errorf("listing flux2 tags: %w", err)
	}

	var matching []*semver.Version
	for _, tag := range tags {
		tv, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}
		if constraint.Check(tv) {
			matching = append(matching, tv)
		}
	}

	if len(matching) == 0 {
		return 0, fmt.Errorf("no flux2 tags match constraint %q", versionExpr)
	}

	sort.Sort(sort.Reverse(semver.Collection(matching)))
	return int(matching[0].Minor()), nil
}

// resolveToTarget parses the version flag value into a patchTarget.
// Accepted formats: "main", "v2.<minor>", "2.<minor>", "<minor>".
func resolveToTarget(vFlag string) (patchTarget, error) {
	if vFlag == "main" {
		return patchTarget{isMain: true}, nil
	}

	s := strings.TrimPrefix(vFlag, "v")

	if strings.Contains(s, ".") {
		parts := strings.SplitN(s, ".", 2)
		if parts[0] != "2" {
			return patchTarget{}, fmt.Errorf("unsupported major version %q, only major version 2 is supported", parts[0])
		}
		minor, err := strconv.Atoi(parts[1])
		if err != nil {
			return patchTarget{}, fmt.Errorf("invalid minor version %q: %w", parts[1], err)
		}
		return patchTarget{fluxMinor: minor}, nil
	}

	minor, err := strconv.Atoi(s)
	if err != nil {
		return patchTarget{}, fmt.Errorf("invalid version %q: expected 'main', 'v2.<minor>', or '<minor>'", vFlag)
	}
	return patchTarget{fluxMinor: minor}, nil
}

// buildCRDURLs constructs the source and target URLs for a CRD file
// based on the controller name, version information, and target.
func buildCRDURLs(
	controller string,
	group string,
	plural string,
	fromMinor int,
	target patchTarget,
) (string, string, error) {
	major, err := version.RepoMajor(controller)
	if err != nil {
		return "", "", err
	}

	ctrlFromMinor, err := version.RepoMinorForFluxMinor(controller, fromMinor)
	if err != nil {
		return "", "", err
	}

	crdFile := fmt.Sprintf("%s_%s.yaml", group, plural)
	fromRef := fmt.Sprintf("v%d.%d.0", major, ctrlFromMinor)
	sourceURL := fmt.Sprintf("%s/%s/blob/%s/config/crd/bases/%s",
		fluxControllerBaseURL, controller, fromRef, crdFile)

	var targetURL string
	if target.isMain {
		targetURL = fmt.Sprintf("%s/%s/blob/main/config/crd/bases/%s",
			fluxControllerBaseURL, controller, crdFile)
	} else {
		ctrlToMinor, err := version.RepoMinorForFluxMinor(controller, target.fluxMinor)
		if err != nil {
			return "", "", err
		}
		toRef := fmt.Sprintf("v%d.%d.0", major, ctrlToMinor)
		targetURL = fmt.Sprintf("%s/%s/blob/%s/config/crd/bases/%s",
			fluxControllerBaseURL, controller, toRef, crdFile)
	}

	return sourceURL, targetURL, nil
}

// computeCRDPatch diffs two CRD YAML files (given as URLs or local paths)
// and returns a kustomize patch targeting the named CRD, or nil if unchanged.
func computeCRDPatch(
	ctx context.Context,
	sourceURL string,
	targetURL string,
	group string,
	plural string,
) (*kustomize.Patch, error) {
	diff, err := diffYAML(ctx, sourceURL, targetURL)
	if err != nil {
		return nil, err
	}

	if len(diff) == 0 {
		return nil, nil
	}

	patchYAML, err := yaml.Marshal(diff)
	if err != nil {
		return nil, fmt.Errorf("marshalling CRD patch: %w", err)
	}

	return &kustomize.Patch{
		Patch: string(patchYAML),
		Target: &kustomize.Selector{
			Kind: "CustomResourceDefinition",
			Name: fmt.Sprintf("%s.%s", plural, group),
		},
	}, nil
}

// computeImagePatch generates a kustomize patch that sets the controller
// Deployment image to the target version.
func computeImagePatch(
	ctx context.Context,
	controller string,
	target patchTarget,
	registry string,
) (*kustomize.Patch, error) {
	var imageTag string
	if target.isMain {
		rootCmd.PrintErrln(`◎`, fmt.Sprintf("  Resolving main branch SHA for %s", controller))
		sha, err := resolveMainBranchSHA(ctx, controller)
		if err != nil {
			return nil, fmt.Errorf("resolving main branch SHA for %s: %w", controller, err)
		}
		imageTag = fmt.Sprintf("rc-%s", sha[:8])
		rootCmd.PrintErrln(`✔`, fmt.Sprintf("  Using image tag %s", imageTag))
		registry = "ghcr.io/fluxcd"
	} else {
		major, err := version.RepoMajor(controller)
		if err != nil {
			return nil, err
		}
		ctrlToMinor, err := version.RepoMinorForFluxMinor(controller, target.fluxMinor)
		if err != nil {
			return nil, err
		}
		imageTag = fmt.Sprintf("v%d.%d.0", major, ctrlToMinor)
	}

	image := fmt.Sprintf("%s/%s:%s", registry, controller, imageTag)

	ops := []map[string]any{
		{
			"op":    "replace",
			"path":  "/spec/template/spec/containers/0/image",
			"value": image,
		},
	}

	patchYAML, err := yaml.Marshal(ops)
	if err != nil {
		return nil, fmt.Errorf("marshalling image patch: %w", err)
	}

	return &kustomize.Patch{
		Patch: string(patchYAML),
		Target: &kustomize.Selector{
			Kind: "Deployment",
			Name: controller,
		},
	}, nil
}

// newGitHubClient creates a GitHub client with optional authentication.
// It uses the go-gh auth package which supports GITHUB_TOKEN, GH_TOKEN
// environment variables and credentials stored by 'gh auth login'.
func newGitHubClient(ctx context.Context) *github.Client {
	token, _ := ghauth.TokenForHost("github.com")
	if token == "" {
		return github.NewClient(nil)
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	return github.NewClient(oauth2.NewClient(ctx, ts))
}

// resolveMainBranchSHA returns the commit SHA of the main branch
// for the given controller repository under the fluxcd GitHub org.
// It is a variable so tests can override it.
var resolveMainBranchSHA = func(ctx context.Context, controller string) (string, error) {
	ghClient := newGitHubClient(ctx)
	branch, _, err := ghClient.Repositories.GetBranch(ctx, "fluxcd", controller, "main", 0)
	if err != nil {
		return "", err
	}
	return branch.GetCommit().GetSHA(), nil
}

// listFlux2Tags fetches all tags from the fluxcd/flux2 GitHub repository.
func listFlux2Tags(ctx context.Context, client *github.Client) ([]string, error) {
	opts := &github.ListOptions{PerPage: 100}
	var allTags []string

	for {
		tags, resp, err := client.Repositories.ListTags(ctx, "fluxcd", "flux2", opts)
		if err != nil {
			return nil, err
		}

		for _, tag := range tags {
			allTags = append(allTags, tag.GetName())
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allTags, nil
}
