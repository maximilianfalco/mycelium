#!/bin/bash
# Mycelium MCP Benchmark
# Runs the same prompts with and without Mycelium MCP on both sonnet and opus.
# Compares token usage, cost, latency, and answer quality.

set -euo pipefail

DIR="$(cd "$(dirname "$0")/.." && pwd)"
RESULTS_DIR="$DIR/benchmark-results"
TARGET_DIR="${1:-.}"
BUDGET="3.00"
MODELS=("sonnet" "opus")

# --- Interactive prompt entry ---

echo "Mycelium MCP Benchmark"
echo "======================"
echo ""
echo "Enter up to 5 prompts (one per line). Press enter on an empty line when done."
echo ""

PROMPTS=()
PROMPT_LABELS=()

for i in 1 2 3 4 5; do
  printf "  [%d/5] > " "$i"
  read -r line
  if [ -z "$line" ]; then
    break
  fi
  PROMPTS+=("$line")
  PROMPT_LABELS+=("prompt-$i")
done

if [ ${#PROMPTS[@]} -eq 0 ]; then
  echo "No prompts entered. Exiting."
  exit 0
fi

total=${#PROMPTS[@]}
model_count=${#MODELS[@]}
grand_total=$((total * model_count * 2))

echo ""
echo "  Prompts:   $total"
echo "  Models:    ${MODELS[*]}"
echo "  Runs:      $grand_total ($total prompts x $model_count models x 2 modes)"
echo "  Budget:    \$$BUDGET per run"
echo "  Directory: $(cd "$TARGET_DIR" && pwd)"
echo ""
printf "Start benchmark? [Y/n] "
read -r confirm
if [ -n "$confirm" ] && [ "$confirm" != "Y" ] && [ "$confirm" != "y" ]; then
  echo "Cancelled."
  exit 0
fi

echo ""

TIMESTAMP="$(date +%Y-%m-%dT%H-%M-%S)-$$"
RUN_DIR="$RESULTS_DIR/$TIMESTAMP"
mkdir -p "$RUN_DIR"

current=0

# Save prompts metadata for the summary generator
python3 -c "
import json, sys
prompts = []
for line in sys.stdin:
    parts = line.strip().split('\t', 1)
    prompts.append({'label': parts[0], 'text': parts[1]})
with open('$RUN_DIR/prompts.json', 'w') as f:
    json.dump({'models': ['sonnet', 'opus'], 'prompts': prompts, 'target': '$(cd "$TARGET_DIR" && pwd)'}, f, indent=2)
" <<EOF
$(for i in "${!PROMPTS[@]}"; do echo "${PROMPT_LABELS[$i]}	${PROMPTS[$i]}"; done)
EOF

SPINNER_PID=""
cleanup() {
  if [ -n "$SPINNER_PID" ]; then
    kill "$SPINNER_PID" 2>/dev/null
    wait "$SPINNER_PID" 2>/dev/null
    printf "\r\033[K" >&2
  fi
}
trap cleanup EXIT INT TERM

spinner() {
  local chars='⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏'
  local start=$SECONDS
  while true; do
    for (( j=0; j<${#chars}; j++ )); do
      local elapsed=$(( SECONDS - start ))
      printf "\r  %s %ds " "${chars:$j:1}" "$elapsed" >&2
      sleep 0.1
    done
  done
}

run_claude() {
  local prompt="$1"
  local mode="$2"    # "with-mcp" or "without-mcp"
  local model="$3"
  local outfile="$4"

  local mcp_args=()
  local full_prompt=""

  if [ "$mode" = "with-mcp" ]; then
    # Uses globally configured Mycelium MCP
    full_prompt="USING MYCELIUM. ${prompt}"
  else
    # --strict-mcp-config blocks all MCP servers not passed via --mcp-config
    mcp_args=(--strict-mcp-config)
    full_prompt="DO NOT USE MYCELIUM. ${prompt}"
  fi

  local exit_code=0
  (cd "$TARGET_DIR" && CLAUDECODE= claude --print \
    --output-format json \
    --dangerously-skip-permissions \
    --max-budget-usd "$BUDGET" \
    --model "$model" \
    --no-session-persistence \
    ${mcp_args[@]+"${mcp_args[@]}"} \
    "$full_prompt" \
  ) > "$outfile" 2>"${outfile%.json}.err" || exit_code=$?
  if [ ! -s "$outfile" ]; then
    echo '{"result":"ERROR","error":"No output produced","exit_code":'"$exit_code"'}' > "$outfile"
  fi
}

extract_field() {
  local file="$1"
  local field="$2"
  python3 -c "
import json, sys
try:
    d = json.load(open('$file'))
    val = d
    for k in '$field'.split('.'):
        val = val.get(k, {}) if isinstance(val, dict) else {}
    print(val if val != {} else 'N/A')
except:
    print('N/A')
"
}

extract_result() {
  local file="$1"
  python3 -c "
import json, sys
try:
    d = json.load(open('$file'))
    print(d.get('result', 'N/A'))
except:
    print('N/A')
"
}

for model in "${MODELS[@]}"; do
  for i in "${!PROMPTS[@]}"; do
    prompt="${PROMPTS[$i]}"
    label="${PROMPT_LABELS[$i]}"
    num=$(printf "%02d" $((i + 1)))
    prefix="${num}-${label}"

    for mode in "with-mcp" "without-mcp"; do
      current=$((current + 1))
      echo "[$current/$grand_total] $model / $mode / $label"

      outfile="$RUN_DIR/${prefix}-${model}-${mode}.json"

      spinner &
      SPINNER_PID=$!
      run_claude "$prompt" "$mode" "$model" "$outfile"
      kill "$SPINNER_PID" 2>/dev/null || true
      wait "$SPINNER_PID" 2>/dev/null || true
      SPINNER_PID=""
      printf "\r\033[K" >&2

      # Extract answer text
      extract_result "$outfile" > "$RUN_DIR/${prefix}-${model}-${mode}.txt"

      # Print quick stats
      cost=$(extract_field "$outfile" "total_cost_usd")
      duration=$(extract_field "$outfile" "duration_ms")
      turns=$(extract_field "$outfile" "num_turns")
      echo "  cost=\$$cost  time=${duration}ms  turns=$turns"
    done
  done
done

echo ""
echo "Generating summary..."

# Generate summary markdown
python3 "$DIR/scripts/summarize.py" "$RUN_DIR"

echo ""
echo "Done! Results in: $RUN_DIR"
echo "Open $RUN_DIR/summary.md to compare results."
