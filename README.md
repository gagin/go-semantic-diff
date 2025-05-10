# Go Semantic Diff Prototype

This prototype demonstrates the core logic for a semantic diff tool in Go.
It stubs out the actual AI model-based embedding generation and uses
text-based similarity (Levenshtein distance) as a placeholder for "semantic" comparison.

## Files

- `main.go`: Main application logic, CLI parsing.
- `diffengine.go`: Core diffing logic (identifying added, deleted, modified blocks).
- `contentblock.go`: Definition of content blocks, text segmentation, and normalization.
- `similarity.go`: Stubbed semantic similarity functions (currently uses Levenshtein).
- `go.mod`, `go.sum`: Go module files.
- `fileA.txt`, `fileB.txt`: Sample input files for testing.

## How to Build and Run

1.  **Ensure Go is installed** (version 1.18 or higher recommended).
2.  **Navigate to this directory** (`go-semantic-diff`).
3.  **Tidy dependencies** (this will download the `levenshtein` package):
    ```bash
    go mod tidy
    ```
4.  **Build the executable:**
    ```bash
    go build .
    ```
    This will create an executable named `go-semantic-diff` (or `go-semantic-diff.exe` on Windows).
5.  **Run the diff tool:**
    ```bash
    ./go-semantic-diff fileA.txt fileB.txt
    ```
    Or, with your own files:
    ```bash
    ./go-semantic-diff path/to/your/file1.txt path/to/your/file2.txt
    ```

## Output Interpretation

The tool will output:
- The number of blocks identified in each file.
- A list of differences, categorized as:
  - `[ADDED]`: Content present in File B but not File A.
  - `[DELETED]`: Content present in File A but not File B.
  - `[MODIFIED]`: Content present in both files but with "semantic" differences (based on the stubbed similarity). Shows a similarity score.
  - `[UNCHANGED]`: Content blocks that are identical in both files. It will also note if their original block IDs or line numbers differ, hinting at a positional shift (a very naive "moved" detection for this prototype).

## Next Steps (for a full tool)

- Integrate actual HuggingFace embedding models (e.g., by calling a Python script/service or using Go bindings for ONNX/GGML if available for Nomic-Embed).
- Implement a more sophisticated "Moved" block detection (Level 3 diff).
- Develop a two-panel GUI for visualization.
- Enhance text segmentation (e.g., for different file types like code via ASTs).
