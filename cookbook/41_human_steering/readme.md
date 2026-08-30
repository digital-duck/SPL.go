# Recipe 41: Human Steering (Human-in-the-Loop)

## Description
This recipe demonstrates the "Pause for Approval" pattern in SPL. It allows a workflow to stop and ingest human feedback through a deterministic `CALL` statement before proceeding to a probabilistic `GENERATE` step. This is essential for safety-critical applications or high-fidelity content creation.

## Key Features
- **Human-in-the-Loop:** Uses a `CALL wait_for_human_feedback()` to pause execution.
- **Conditional Refinement:** Branches the logic based on whether feedback was received.
- **Cost Efficiency:** Only triggers the second `GENERATE` if human steering is required.

## Usage
```bash
spl-go run cookbook-v3.0/41_human_steering/human_steering.spl --tools cookbook-v3.0/tools.py
```
