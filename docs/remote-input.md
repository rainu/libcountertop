# Safe remote input primitives

Consumers may use touch.Cancel to release a synthetic contact without completing
a tap, hold, swipe or back gesture. The recognizer, screen renderer, touch marks,
message drag and pending press state are cleaned without invoking the pending
click. Existing Up semantics are unchanged.

textentry.Controller.CallGuarded evaluates a local policy and the current focused
View on the UI queue immediately before an edit. This prevents a queued remote
editor operation from outliving its permission or being redirected into a newly
focused password field. Guards must not retain private Views or recursively call
the same Controller. Ordinary Call behavior is unchanged.

Used by the separately permission-gated desktop controller in rainu/echolocal#15.
