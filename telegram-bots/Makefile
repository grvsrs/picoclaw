.PHONY: install test lint clean setup check

VENV     := .venv
PYTHON   := $(VENV)/bin/python
PIP      := $(VENV)/bin/pip
PYTEST   := $(VENV)/bin/pytest

# ---- Install ----

install: $(VENV)/pyvenv.cfg

$(VENV)/pyvenv.cfg: requirements.txt
	python3 -m venv $(VENV)
	$(PIP) install --upgrade pip --quiet
	$(PIP) install --quiet -r requirements.txt
	@touch $@

# ---- Test ----

test: install
	$(PYTEST) tests/ -v --tb=short

# ---- Lint ----

lint: install
	$(PYTHON) -m py_compile bots/bot_base.py
	$(PYTHON) -m py_compile bots/dev_bot.py
	$(PYTHON) -m py_compile bots/ops_bot.py
	$(PYTHON) -m py_compile bots/monitor_bot.py
	$(PYTHON) -m py_compile bots/executor.py
	@echo "All files compile OK."

# ---- Setup (full Ubuntu 22.04 install) ----

setup:
	chmod +x setup.sh
	./setup.sh

# ---- Check (lint + test) ----

check: lint test

# ---- Clean ----

clean:
	rm -rf $(VENV) __pycache__ .pytest_cache
	find . -name "*.pyc" -delete
	find . -name "__pycache__" -type d -exec rm -rf {} + 2>/dev/null || true
