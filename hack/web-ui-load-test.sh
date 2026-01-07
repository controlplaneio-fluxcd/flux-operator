#!/usr/bin/env bash

# Copyright 2026 Stefan Prodan.
# SPDX-License-Identifier: AGPL-3.0

# Load test script for Flux Status Page web server endpoints.
# Discovers resources from the cluster using the CLI and tests
# /api/v1/resources and /api/v1/resource endpoints.

set -euo pipefail

REPOSITORY_ROOT=$(git rev-parse --show-toplevel)

# Configuration (can be overridden via environment variables)
CLI="${FO_CLI:-${REPOSITORY_ROOT}/bin/flux-operator-cli}"
BASE_URL="${WEB_BASE_URL:-http://localhost:9080}"
BASE_URL="${BASE_URL%/}"
CONCURRENCY="${WEB_CONCURRENCY:-5}"
REQUESTS="${WEB_REQUESTS:-10}"

# Temp directory for results
RESULTS_DIR=$(mktemp -d)
trap "rm -rf ${RESULTS_DIR}" EXIT

# Colors (disabled if not a terminal)
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[0;33m'
    CYAN='\033[0;36m'
    BOLD='\033[1m'
    NC='\033[0m' # No Color
else
    RED=''
    GREEN=''
    YELLOW=''
    CYAN=''
    BOLD=''
    NC=''
fi

# Global counters
CURRENT_TEST=0
TOTAL_TESTS=0
GLOBAL_MIN=999999
GLOBAL_MAX=0
GLOBAL_TOTAL_TIME=0
GLOBAL_TOTAL_REQUESTS=0

info() {
    echo -e "${CYAN}▶${NC} $*"
}

fatal() {
    echo -e "${RED}✗ ERROR:${NC} $*" >&2
    exit 1
}

print_header() {
    echo ""
    echo -e "${BOLD}$1${NC}"
    printf '─%.0s' {1..95}
    echo ""
    printf "${BOLD}%-55s %5s %5s %5s %7s %7s %7s${NC}\n" "Endpoint" "Reqs" "OK" "Fail" "Avg" "Min" "Max"
    printf '─%.0s' {1..95}
    echo ""
}

print_summary_box() {
    local total_success=$1
    local total_failed=$2
    local elapsed=$3
    local avg_latency=$4
    local min_latency=$5
    local max_latency=$6

    echo ""
    printf '═%.0s' {1..95}
    echo ""
    echo -e "${BOLD} SUMMARY${NC}"
    printf '═%.0s' {1..95}
    echo ""

    local status_color="${GREEN}"
    if [ "${total_failed}" -gt 0 ]; then
        status_color="${RED}"
    fi

    printf " Total Requests: ${BOLD}%d${NC}" "${GLOBAL_TOTAL_REQUESTS}"
    printf "    Success: ${GREEN}%d${NC}" "${total_success}"
    printf "    Failed: ${status_color}%d${NC}" "${total_failed}"
    printf "    Time: ${BOLD}%ds${NC}\n" "${elapsed}"

    printf " Avg Latency: ${BOLD}%dms${NC}" "${avg_latency}"
    printf "   Min: ${BOLD}%dms${NC}" "${min_latency}"
    printf "   Max: ${BOLD}%dms${NC}\n" "${max_latency}"

    printf '═%.0s' {1..95}
    echo ""
}

# Run load test for a single endpoint
# Args: $1=description $2=url
load_test_endpoint() {
    local desc="$1"
    local url="$2"
    local results_file="${RESULTS_DIR}/results.txt"

    ((CURRENT_TEST++)) || true

    # Generate URLs for xargs
    # Capture: http_code, response_size, time_total
    : > "${results_file}"
    for ((i=0; i<REQUESTS; i++)); do
        echo "${url}"
    done | xargs -P "${CONCURRENCY}" -I {} \
        curl -s -o /dev/null -w "%{http_code} %{size_download} %{time_total}\n" "{}" >> "${results_file}" 2>/dev/null

    # Calculate statistics
    local success=0 failed=0 total_time=0 min_time=999999 max_time=0 count=0
    while read -r status size time_sec; do
        [ -z "${status}" ] && continue
        local duration
        duration=$(awk "BEGIN {printf \"%.0f\", ${time_sec} * 1000}")
        # Count as success only if status=200 and response is not empty ({} = 2 bytes)
        if [ "${status}" = "200" ] && [ "${size}" -gt 2 ]; then
            ((success++)) || true
        else
            ((failed++)) || true
        fi
        ((total_time += duration)) || true
        if [ "${duration}" -lt "${min_time}" ]; then
            min_time=${duration}
        fi
        if [ "${duration}" -gt "${max_time}" ]; then
            max_time=${duration}
        fi
        ((count++)) || true
    done < "${results_file}"

    local avg_time=0
    if [ ${count} -gt 0 ]; then
        avg_time=$((total_time / count))
    fi
    if [ ${min_time} -eq 999999 ]; then
        min_time=0
    fi

    # Update global stats
    ((GLOBAL_TOTAL_TIME += total_time)) || true
    ((GLOBAL_TOTAL_REQUESTS += count)) || true
    if [ "${min_time}" -lt "${GLOBAL_MIN}" ]; then
        GLOBAL_MIN=${min_time}
    fi
    if [ "${max_time}" -gt "${GLOBAL_MAX}" ]; then
        GLOBAL_MAX=${max_time}
    fi

    # Truncate description if too long
    local short_desc="${desc}"
    if [ ${#desc} -gt 47 ]; then
        short_desc="${desc:0:44}..."
    fi

    # Print result row with color
    local status_icon="${GREEN}✓${NC}"
    local fail_color="${NC}"
    if [ "${failed}" -gt 0 ]; then
        status_icon="${RED}✗${NC}"
        fail_color="${RED}"
    fi

    printf "[%2d/%2d] %-47s %5d %5d ${fail_color}%5d${NC} %6dms %6dms %6dms %b\n" \
        "${CURRENT_TEST}" "${TOTAL_TESTS}" "${short_desc}" \
        "${count}" "${success}" "${failed}" \
        "${avg_time}" "${min_time}" "${max_time}" "${status_icon}"

    # Store counts for totals
    echo "${success} ${failed}" >> "${RESULTS_DIR}/totals.txt"
}

# Main script
main() {
    echo ""
    echo -e "${BOLD}Flux Status Page Load Test${NC}"
    echo -e "Target: ${CYAN}${BASE_URL}${NC}"
    echo -e "Config: ${CONCURRENCY} concurrent, ${REQUESTS} requests/endpoint"

    # Check CLI exists
    if [ ! -x "${CLI}" ]; then
        fatal "CLI not found at ${CLI}. Run 'make cli-build' first."
    fi

    # Check server is reachable
    info "Checking server connectivity..."
    if ! curl -s "${BASE_URL}/" | grep -q "Flux"; then
        fatal "Server not reachable at ${BASE_URL}"
    fi

    # Discover resources from cluster
    info "Discovering resources from cluster..."
    RESOURCES_JSON=$("${CLI}" get all -A --output json 2>/dev/null)

    if [ -z "${RESOURCES_JSON}" ] || [ "${RESOURCES_JSON}" = "[]" ]; then
        fatal "No resources found in cluster"
    fi

    # Extract unique kinds
    KINDS=$(echo "${RESOURCES_JSON}" | jq -r '.[].kind' | sort -u)
    KINDS_COUNT=$(echo "${KINDS}" | wc -l | tr -d ' ')

    # Extract unique namespaces
    NAMESPACES=$(echo "${RESOURCES_JSON}" | jq -r '.[].name | split("/")[0]' | sort -u)
    NS_COUNT=$(echo "${NAMESPACES}" | wc -l | tr -d ' ')

    # Extract resource tuples (kind|namespace|name)
    RESOURCES=$(echo "${RESOURCES_JSON}" | jq -r '.[] | "\(.kind)|\(.name | split("/")[0])|\(.name | split("/")[1])"')
    RES_COUNT=$(echo "${RESOURCES}" | wc -l | tr -d ' ')

    # Calculate total tests: 1 (no filter) + kinds + namespaces + 2 (status) + resources
    TOTAL_TESTS=$((1 + KINDS_COUNT + NS_COUNT + 2 + RES_COUNT))

    info "Found ${KINDS_COUNT} kinds, ${NS_COUNT} namespaces, ${RES_COUNT} resources (${TOTAL_TESTS} tests)"

    # Start timing after discovery
    local start_time
    start_time=$(date +%s)

    # Initialize totals file
    > "${RESULTS_DIR}/totals.txt"

    # Test /api/v1/resources endpoint
    print_header "Testing /api/v1/resources"

    load_test_endpoint "/api/v1/resources (no filter)" "${BASE_URL}/api/v1/resources"

    # Test by kind
    for kind in ${KINDS}; do
        load_test_endpoint "?kind=${kind}" "${BASE_URL}/api/v1/resources?kind=${kind}"
    done

    # Test by namespace
    for ns in ${NAMESPACES}; do
        load_test_endpoint "?namespace=${ns}" "${BASE_URL}/api/v1/resources?namespace=${ns}"
    done

    # Test by status
    for status in Ready Failed; do
        load_test_endpoint "?status=${status}" "${BASE_URL}/api/v1/resources?status=${status}"
    done

    # Test /api/v1/resource endpoint
    print_header "Testing /api/v1/resource"

    # Test each discovered resource
    while IFS='|' read -r kind ns name; do
        load_test_endpoint "${kind}/${ns}/${name}" "${BASE_URL}/api/v1/resource?kind=${kind}&namespace=${ns}&name=${name}"
    done <<< "${RESOURCES}"

    # Calculate totals
    local total_success=0 total_failed=0
    while read -r success failed; do
        ((total_success += success)) || true
        ((total_failed += failed)) || true
    done < "${RESULTS_DIR}/totals.txt"

    local end_time elapsed_time
    end_time=$(date +%s)
    elapsed_time=$((end_time - start_time))

    # Calculate global average
    local global_avg=0
    if [ "${GLOBAL_TOTAL_REQUESTS}" -gt 0 ]; then
        global_avg=$((GLOBAL_TOTAL_TIME / GLOBAL_TOTAL_REQUESTS))
    fi
    if [ "${GLOBAL_MIN}" -eq 999999 ]; then
        GLOBAL_MIN=0
    fi

    print_summary_box "${total_success}" "${total_failed}" "${elapsed_time}" \
        "${global_avg}" "${GLOBAL_MIN}" "${GLOBAL_MAX}"
}

main "$@"
