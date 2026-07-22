# UI screenshots

Captured from the React frontend (`frontend/`) driving the live Go API, walking
one task through the whole approval workflow.

| File | What it shows |
| --- | --- |
| `bob-submitted.png` | Bob (member) after submitting. Status is `submitted` and the Submit button is gone: the state machine allows no second submit. **No Review queue tab** — members do not get one. |
| `alice-queue.png` | Alice (admin) in the review queue. Bob's task with Approve / Reject, and the unread badge showing 1. |
| `alice-notifications.png` | The admin's notification: *Bob submitted "read the pq docs" for review*. |
| `bob-notified.png` | The verdict travelling back: *Alice approved your task "read the pq docs"*. |

The Review queue tab is hidden for members, but that is convenience only. The
real guard is `RequireAdmin` on the server, which answers 403 to a member who
calls `/api/admin/tasks` directly.

Regenerate with the API and `npm run dev` both running, saving to **absolute**
paths (a relative path goes to the browser daemon's working directory, not
yours):

```bash
agent-browser open http://localhost:5173
agent-browser screenshot "$PWD/docs/screenshots/name.png"
```
