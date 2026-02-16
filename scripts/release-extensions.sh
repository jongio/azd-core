#!/usr/bin/env bash
set -euo pipefail

# --- Color helpers (graceful fallback if tput unavailable) ---
if command -v tput &>/dev/null && tput colors &>/dev/null; then
    BOLD=$(tput bold)
    CYAN=$(tput setaf 6)
    GREEN=$(tput setaf 2)
    YELLOW=$(tput setaf 3)
    RED=$(tput setaf 1)
    RESET=$(tput sgr0)
else
    BOLD="" CYAN="" GREEN="" YELLOW="" RED="" RESET=""
fi

# --- Validation ---
if [[ $# -lt 1 ]]; then
    echo "${RED}Usage: $0 <version>${RESET}"
    echo "  Example: $0 v0.5.0"
    exit 1
fi

VERSION="$1"

if [[ ! "$VERSION" =~ ^v ]]; then
    echo "${RED}❌ Version must start with 'v' (e.g. v0.5.0)${RESET}"
    exit 1
fi

if ! command -v gh &>/dev/null; then
    echo "${RED}❌ 'gh' CLI is not installed. Install it from https://cli.github.com${RESET}"
    exit 1
fi

if ! gh auth status &>/dev/null 2>&1; then
    echo "${RED}❌ 'gh' CLI is not authenticated. Run 'gh auth login' first.${RESET}"
    exit 1
fi

# --- Config ---
REPOS=(
    "jongio/azd-app"
    "jongio/azd-copilot"
    "jongio/azd-exec"
    "jongio/azd-rest"
)

declare -a DISPATCHED=()

echo ""
echo "${BOLD}${CYAN}📦 azd-core Release Coordinator${RESET}"
echo "${CYAN}   Version: ${BOLD}${VERSION}${RESET}"
echo "${CYAN}   Repos:   ${#REPOS[@]} consumer extensions${RESET}"
echo ""

# --- Process each repo ---
for REPO in "${REPOS[@]}"; do
    echo "${BOLD}${YELLOW}🔄 ${REPO}${RESET}"

    # Fetch open PRs
    PRS_JSON=$(gh pr list --repo "$REPO" --state open --json number,title,headRefName --limit 20 2>&1) || {
        echo "  ${RED}Failed to list PRs: ${PRS_JSON}${RESET}"
        echo ""
        continue
    }

    PR_COUNT=$(echo "$PRS_JSON" | jq length)

    if [[ "$PR_COUNT" -eq 0 ]]; then
        echo "  ${YELLOW}⏭️  No open PRs, skipping${RESET}"
        echo ""
        continue
    fi

    # Display numbered list
    echo "  ${BOLD}Open PRs:${RESET}"
    for i in $(seq 0 $((PR_COUNT - 1))); do
        PR_NUM=$(echo "$PRS_JSON" | jq -r ".[$i].number")
        PR_TITLE=$(echo "$PRS_JSON" | jq -r ".[$i].title")
        PR_BRANCH=$(echo "$PRS_JSON" | jq -r ".[$i].headRefName")
        echo "  ${CYAN}[$((i + 1))]${RESET} #${PR_NUM} ${PR_TITLE} ${YELLOW}(${PR_BRANCH})${RESET}"
    done

    # Prompt for selection
    echo ""
    read -rp "  Select PR number to update (or 's' to skip): " SELECTION

    if [[ "$SELECTION" == "s" || "$SELECTION" == "S" ]]; then
        echo "  ${YELLOW}⏭️  Skipped${RESET}"
        echo ""
        continue
    fi

    # Validate selection is a number in range
    if ! [[ "$SELECTION" =~ ^[0-9]+$ ]] || [[ "$SELECTION" -lt 1 ]] || [[ "$SELECTION" -gt "$PR_COUNT" ]]; then
        echo "  ${RED}❌ Invalid selection, skipping${RESET}"
        echo ""
        continue
    fi

    INDEX=$((SELECTION - 1))
    SELECTED_PR=$(echo "$PRS_JSON" | jq -r ".[$INDEX].number")
    SELECTED_TITLE=$(echo "$PRS_JSON" | jq -r ".[$INDEX].title")

    echo "  Triggering update-azd-core.yml for PR #${SELECTED_PR}..."

    gh workflow run update-azd-core.yml \
        --repo "$REPO" \
        -f version="$VERSION" \
        -f pr_number="$SELECTED_PR"

    echo "  ${GREEN}✅ Dispatched: ${REPO} PR #${SELECTED_PR} (${SELECTED_TITLE})${RESET}"
    DISPATCHED+=("${REPO} PR #${SELECTED_PR} — ${SELECTED_TITLE}")
    echo ""
done

# --- Summary ---
echo "${BOLD}${CYAN}────────────────────────────────────${RESET}"
echo "${BOLD}${CYAN}📦 Summary for ${VERSION}${RESET}"
echo "${BOLD}${CYAN}────────────────────────────────────${RESET}"

if [[ ${#DISPATCHED[@]} -eq 0 ]]; then
    echo "  ${YELLOW}No workflows dispatched.${RESET}"
else
    for ENTRY in "${DISPATCHED[@]}"; do
        echo "  ${GREEN}✅ ${ENTRY}${RESET}"
    done
fi

echo ""
echo "${GREEN}Done!${RESET}"
