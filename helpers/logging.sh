command -v gum >/dev/null || {
    echo "Outpost requires Gum. Re-run the Outpost installer." >&2
    exit 1
}

outpost_info() {
    gum log --level info "$*"
}

outpost_error() {
    gum log --level error "$*" >&2
}
