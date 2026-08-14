**Race Conditions**

Two threads walk into a shared bar,
each certain they arrived there first —
no lock upon the door ajar,
no mutex there to quench the thirst.

They read the same stale count at once,
increment, and write it back;
one thread's gain, by pure mischance,
is swallowed by the other's stack.

We dream in locks and semaphores,
in queues that never lose their place,
in atoms guarding shared-state doors
so no two writers interlace.

Yet still, some nights, the deadlock waits —
two threads, each holding half a key,
politely stalled at closed-door gates,
concurrent, correct... eventually.
