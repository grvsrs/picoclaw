"""Shared test fixtures for PicoClaw bot handler tests."""

import os
import sys
from pathlib import Path

import pytest

# Ensure the bots directory is importable
sys.path.insert(0, str(Path(__file__).parent.parent / "bots"))
