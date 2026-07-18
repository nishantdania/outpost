mkdir -p "$HOME/.local/bin"

while IFS= read -r command; do
    ln -sfn "$OUTPOST_PATH/bin/$command" "$HOME/.local/bin/$command"
done < <(find "$OUTPOST_PATH/bin" -maxdepth 1 -type f -executable -printf '%f\n')

outpost_info "Outpost commands are available from $HOME/.local/bin"
"$OUTPOST_PATH/bin/outpost-image-fetch"
