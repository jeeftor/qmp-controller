#!/bin/bash

VMID=""
QMP_BIN="qmp-controller"

# Character sets for each line (expanded)
declare -a TRAINING_LINES=(
    "abcdefghijklmnopqrstuvwxyz"                    # Line 1: lowercase
    "ABCDEFGHIJKLMNOPQRSTUVWXYZ"                    # Line 2: uppercase
    "0123456789"                                    # Line 3: numbers
    "!@#\$%^&*()-_=+[]{}|;':\",./<>?"               # Line 4: punctuation
    "\`~"                                           # Line 5: symbols
    "àáâãäåæçèéêëìíîïñòóôõöøùúûüýÿ"                 # Line 6: European lowercase
    "ÀÁÂÃÄÅÆÇÈÉÊËÌÍÎÏÑÒÓÔÕÖØÙÚÛÜÝŸ"                 # Line 7: European uppercase
    "¡¢£¤¥¦§¨©ª«¬®¯°±²³´µ¶·¸¹º»¼½¾¿"                # Line 8: extended Latin
#    "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0A\x0B\x0C\x0D\x0E\x0F"  # Line 9: Control characters (NULL to SI)
#    "\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1A\x1B\x1C\x1D\x1E\x1F"  # Line 10: Control characters (DLE to US)
    "┌┐└┘├┤┬┴┼═║╒╓╔╕╖╗╘╙╚╛╜╝╞╟╠╡╢╣"            # Line 11: Box drawing (single)
    "═║╤╥╦╧╨╩╪╫╬╱╲╳╴╵╶╷╸╹╺╻╼╽╾╿"                # Line 12: Box drawing (double)
    "░▒▓▁▂▃▄▅▆▇█▉▊▋▌▍▎▏"                        # Line 13: Block elements
    "◀▶▼▲◄►◙◘◗◖◕◔◓◒◑"                            # Line 14: Geometric shapes
    "©®™£€¥¢∞§¶•ªº¬√∑≠≤≥"                        # Line 15: Additional symbols
    "αβγδεζηθικλμνξοπρστυφχψω"                    # Line 16: Greek lowercase
    "ΑΒΓΔΕΖΗΘΙΚΛΜΝΞΟΠΡΣΤΥΦΧΨΩ"                    # Line 17: Greek uppercase
)
