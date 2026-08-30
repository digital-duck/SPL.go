# Recipe 44: Adaptive Failover (Resilience Manager)

## Description
This recipe demonstrates how to build resilient AI workflows using dynamic model switching. It uses a "Flash" model (cheap and fast) for the initial generation and then uses a deterministic `CALL check_quality()` to validate the output. If the quality is insufficient, the workflow automatically reroutes the query to a "Pro" model (more expensive and capable) for a higher-fidelity response.

## Key Features
- **Cost Optimization:** Only uses expensive models when necessary.
- **Dynamic Rerouting:** Automatically switches models based on runtime quality validation.
- **Resilient AI Pipelines:** Prevents "lazy" or incorrect responses by having a fallback mechanism.

## Usage
```bash
spl-go run cookbook-v3.0/44_adaptive_failover/adaptive_failover.spl --tools cookbook-v3.0/tools.py
```
