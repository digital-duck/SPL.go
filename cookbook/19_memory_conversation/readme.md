# Recipe 19: Memory-Augmented Conversation

A conversational agent that loads a persistent user profile and chat history from the SPL memory store, extracts new facts from each turn, and replies with full context.

## Setup

No setup required — the memory store is created automatically under `cookbook/.spl/memory.db` (relative to the project root where `spl run` is invoked).

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `user_input` | TEXT | *(required)* | The user's message for this turn |

## Usage

```bash
# Turn 1: introduce yourself
spl-go run cookbook/19_memory_conversation/memory_chat.spl --adapter ollama \
    user_input="My name is Alice and I'm a data scientist"

# Turn 2: test recall
spl-go run cookbook/19_memory_conversation/memory_chat.spl --adapter ollama \
    user_input="What's my name?"

# Turn 3: add more facts
spl-go run cookbook/19_memory_conversation/memory_chat.spl --adapter ollama \
    user_input="I prefer Python over R"

# Turn 4: review stored profile
spl-go run cookbook/19_memory_conversation/memory_chat.spl --adapter ollama \
    user_input="What do you know about me?"
```

## Memory management

```bash
# Inspect stored profile
spl memory get chat_user_profile

# Reset between sessions
spl memory delete chat_user_profile
spl memory delete chat_history
```

## Note

Memory persistence is fully implemented via the `STORE @var IN memory.<key>` statement. After each turn, the updated profile and chat history are written back to `cookbook/.spl/memory.db` and loaded on the next run.
