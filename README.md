# Go Semantic Diff (go-semantic-diff)

## Original Task & Vision

`go-semantic-diff` is a command-line utility designed to provide a "semantic" difference between two text files. Unlike traditional `diff` tools that are line-order sensitive, this tool aims to understand content similarity even when blocks of text are moved, slightly modified, or surrounded by different "wrapper" text (like headers, footers, or code comments).

The goal is to offer a more intelligent diff that highlights:
1.  **Presence/Absence:** Content truly new or deleted.
2.  **Semantic Modification:** Content that exists in both files but has been altered.
3.  **Structural/Positional Changes:** Content that is identical or similar but appears in a different location or with different surrounding context.

The primary output should be human-readable and focus on meaningful changes, abstracting away minor reordering or purely decorative differences where possible.

## Use Cases

*   **Comparing Code Concatenations:** Analyzing outputs from tools that merge multiple source files, where the order of concatenation or per-file wrappers (e.g., `<file path="...">...</file>` vs. `--- filename ---`) might differ.
*   **Refactored Code Analysis:** Identifying core logic changes when code has been moved between files or functions, or when comments and formatting have changed extensively.
*   **Document Versioning:** Comparing document drafts where paragraphs or sections might be reordered or rephrased.
*   **Configuration File Comparison:** Understanding changes in configuration files where order might not be significant for some blocks, but value changes are.
*   **Identifying Plagiarism or Content Reuse:** Finding significant chunks of identical or very similar text between two large documents.

## Core Processing Logic & Priorities

The diffing process follows a multi-stage approach, prioritizing robust matches first:

1.  **Global "Megablock" Matching (Line-Checksum Based):**
    *   Both input files are initially broken down into individual lines.
    *   Each line is normalized (trimmed, lowercased, multiple spaces collapsed) and a checksum is calculated.
    *   The tool iteratively finds the *longest contiguous sequences of lines* that have identical checksum sequences in both files. These sequences must meet a minimum length (e.g., 3 lines) to be considered a "megablock."
    *   These megablocks are marked as definite `UNCHANGED` anchors. They represent large, identical portions of content present in both files, regardless of their absolute position. Lines consumed by megablocks are excluded from further processing in this stage.

2.  **Gap Segmentation (Paragraph-Based):**
    *   The lines *not* part of any megablock form "gaps" in both files.
    *   The text within these gaps is then segmented into paragraph-like `ContentBlock`s using double newline (`\n\s*\n`) as a separator. Each block is normalized for comparison.

3.  **Semantic Matching of Gap Paragraphs:**
    *   Paragraph blocks from File A's gaps are compared against paragraph blocks from File B's gaps using a semantic similarity metric (currently Levenshtein distance on normalized text).
    *   A similarity threshold (configurable via `--threshold`) determines if two gap paragraphs are considered a `MODIFIED` pair. A heuristic is applied to only attempt semantic matching for paragraphs exceeding a minimum line count (e.g., 3 lines) to avoid spurious matches of very short, common phrases.
    *   Paragraphs are matched greedily: each gap paragraph from File A finds its best available semantic match in File B.

4.  **LIS (Longest Increasing Subsequence) for Positional Analysis:**
    *   All paired blocks (megablocks from Step 1, type `UNCHANGED`; and matched gap paragraphs from Step 3, type `MODIFIED`) are collected.
    *   These pairs are sorted based on the start line of their File A block.
    *   The LIS algorithm is applied to the start lines of the corresponding File B blocks.
    *   Pairs whose File B blocks form part of this LIS are considered "in place." Their type remains `UNCHANGED` or `MODIFIED`.
    *   Pairs *not* in the LIS are re-categorized as `MOVED`. Their original similarity score (if from a semantic match) is retained.

5.  **Identifying Added/Deleted Gap Paragraphs:**
    *   Gap paragraphs from File A that were not part of a megablock and did not find a semantic match become `DELETED`.
    *   Gap paragraphs from File B that were not part of a megablock and did not find a semantic match become `ADDED`.

6.  **Line-Level Diff for Modified Blocks:**
    *   For paragraph blocks ultimately classified as `MODIFIED` (either in-place or moved but with content changes), a secondary line-level diff is performed on their original text using the `diffmatchpatch` library. This provides a detailed breakdown of character/word-level changes *within* those modified paragraphs.

7.  **Output Generation:**
    *   Results are grouped by type (`NEW`, `DELETED`, `MOVED`, `CHANGED`, `UNCHANGED_IN_PLACE`).
    *   A compact summary is shown by default.
    *   The `--details` flag allows users to specify which sections to view in full detail. In detailed view, adjacent or nearly adjacent blocks of the same type are coalesced for readability (see "Coalesced Output" below).
    *   The `--focus` flag reports on the status of a specific line range from File A.

## Features

*   **Megablock Matching:** Identifies large identical sections first to anchor the diff.
*   **Paragraph-Level Semantic Diff:** Compares non-identical sections based on content similarity rather than strict line order.
*   **Levenshtein Distance:** Used for semantic similarity scoring (placeholder for future embedding models).
*   **Moved Block Detection:** Uses LIS to distinguish blocks that changed position from those truly new/deleted or modified in place.
*   **Line-Level Sub-Diffs:** Shows detailed changes within larger "modified" paragraph blocks.
*   **Configurable Similarity Threshold:** `--threshold` flag.
*   **Selective Detailed Output:** `--details` flag (e.g., `new,deleted`, `moved`, `all`).
*   **Focus Mode:** `--focus n,m` flag to query the status of specific lines in File A.
*   **Debug Mode:** `--debug` flag for verbose internal logging.
*   **Coalesced Output:** In detailed views, blocks of the same type that are (nearly) adjacent in their respective source files are grouped. For `NEW` and `DELETED` blocks, this adjacency is determined by their line numbers in the source file, ensuring that only genuinely contiguous new or deleted content is grouped. This prevents misleadingly large line ranges when, for example, a file has a new header and footer but the content in between is matched or moved. For `MODIFIED`, `MOVED`, and `UNCHANGED` blocks, coalescing primarily considers adjacency in File A, and then File B.

## Previously Tried Attempts & Their Drawbacks

1.  **Pure Line-by-Line Segmentation:**
    *   **Attempt:** Treat every line as a block.
    *   **Drawback:** Generated too many blocks, making the diff noisy and slow. Lost semantic context of paragraphs/larger code structures. Coalescing helped but couldn't overcome fundamental granularity if intervening lines had different diff types. For example, an XML header would appear as many small "NEW" blocks interspersed with "MOVED" or "UNCHANGED" lines if those individual lines found matches.

2.  **Pure Paragraph-Based Segmentation (`\n\s*\n`):**
    *   **Attempt:** Treat text between blank lines as blocks.
    *   **Drawback:** Good for prose and some code, but struggled with files without consistent blank line separators (e.g., dense code, some XML). A single line change within a large code block without internal blank lines would mark the entire block as `MODIFIED`, lacking granularity. It also struggled to isolate "wrapper" text (like a single-line header/footer for a code block) if it wasn't separated by a blank line from the code itself.

3.  **Simpler Global Exact Match then Semantic Match (without Megablocks first):**
    *   **Attempt:** First find all paragraph blocks with identical checksums globally. Then, for remaining blocks, do semantic matching.
    *   **Drawback:** Better, but could still lead to issues if a large, truly new block (like a file header) contained phrases or short lines that spuriously matched small, unrelated blocks in the other file with a low-enough similarity threshold. The "Megablock" strategy addresses this by prioritizing large, *contiguous sequences of identical lines* first.

The current multi-pass strategy (Megablocks -> Gap Paragraphs -> LIS -> Sub-diffs) aims to balance finding large structural similarities with providing detailed analysis of differences.

## How to Build and Run

1.  **Ensure Go is installed** (version 1.18 or higher recommended).
2.  **Get Dependencies:**
    ```bash
    go get github.com/sergi/go-diff/diffmatchpatch
    ```
3.  **Navigate to the project directory.**
4.  **Build the executable:**
    ```bash
    go build .
    ```
    This creates an executable named `go-semantic-diff` (or `go-semantic-diff.exe` on Windows).
5.  **Run the diff tool:**
    ```bash
    # Default compact output (details for new/deleted)
    ./go-semantic-diff [--threshold 0.X] <fileA> <fileB>

    # Detailed output for all sections
    ./go-semantic-diff --details=all [--threshold 0.X] <fileA> <fileB>

    # Detailed output for specific sections
    ./go-semantic-diff --details=moved,changed [--threshold 0.X] <fileA> <fileB>

    # Focus on specific lines from File A
    ./go-semantic-diff --focus 10,20 [--threshold 0.X] <fileA> <fileB>

    # Enable debug logging
    ./go-semantic-diff --debug [--details=...] [--threshold 0.X] <fileA> <fileB>
    ```

## Future Enhancements (Ideas)

*   **True Semantic Embeddings:** Replace Levenshtein distance with sentence transformer embeddings (e.g., from HuggingFace, run locally or via API) for more nuanced similarity detection.
*   **Configuration File for Segmentation Strategies:** Allow users to define block delimiters or segmentation rules per file type.
*   **Interactive UI:** A two-panel GUI for visualizing diffs and navigating changes.
*   **Performance Optimization:** For very large files, explore more advanced algorithms for megablock detection or parallel processing.