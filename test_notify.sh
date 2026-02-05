#!/bin/bash

echo "Testing OSC desktop notifications..."
echo "Watch for desktop notifications to appear."
echo ""

# Check if we're in tmux
if [ -n "$TMUX" ]; then
    echo "Detected tmux session - using tmux-wrapped escape codes"
    echo ""
fi

sleep 2

if [ -n "$TMUX" ]; then
    echo "==> Testing OSC 777 in tmux..."
    printf '\033Ptmux;\033\033]777;notify;Test OSC 777 (tmux);This is from hubell test script\007\033\\' > /dev/tty
else
    echo "==> Testing OSC 777 (urxvt, foot, contour)..."
    printf '\033]777;notify;Test OSC 777;This is from hubell test script\007' > /dev/tty
fi
echo ""
sleep 2

if [ -n "$TMUX" ]; then
    echo "==> Testing OSC 9 in tmux..."
    printf '\033Ptmux;\033\033]9;Test OSC 9 from hubell (tmux)\007\033\\' > /dev/tty
else
    echo "==> Testing OSC 9 (iTerm2, mintty, ConEmu)..."
    printf '\033]9;Test OSC 9 from hubell\007' > /dev/tty
fi
echo ""
sleep 2

echo "==> Testing OSC 99..."
printf '\033]99;i=1:d=0;Test OSC 99 from hubell\033\\' > /dev/tty
echo ""
sleep 2

echo ""
echo "==> Testing macOS native notification (if available)..."
if command -v osascript &> /dev/null; then
    osascript -e 'display notification "This is a native macOS notification" with title "hubell test"'
    echo "Sent native macOS notification"
else
    echo "osascript not available (not on macOS)"
fi

echo ""
echo "Test complete!"
echo ""
echo "Terminal compatibility:"
echo "  • OSC 777: urxvt, foot, contour"
echo "  • OSC 9: iTerm2, mintty, ConEmu, Windows Terminal"
echo "  • OSC 99: tmux"
echo "  • Native: macOS via osascript"
