#!/bin/bash

# Define the comment and the lines to add
COMMENT="# << lazywal autocompletion <<<<"
LINE1="complete -C lazywal lazywal"

# Check if the comment already exists in .bashrc
if ! grep -Fxq "$COMMENT" ~/.bashrc; then
    # If the comment does not exist, add the comment and the autocompletion line
    echo "$COMMENT" >> ~/.bashrc
    echo "$LINE1" >> ~/.bashrc
else
    echo "Auto-completion already added in ~/.bashrc"
fi

