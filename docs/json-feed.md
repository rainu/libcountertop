# JSON Feed media provider

JSON Feed 1.1 is an optional provider in the **existing Photos application**.
It does not import an external media library or require a service-specific API.

Configure Photos → JSON Feed on the device, or the Photos settings in the web UI:
1. Enter an absolute HTTP(S) feed URL (ordinary query parameters are opaque).
   The URL is a masked secret; it is not displayed in status or published to HA.
2. Enable the provider. Existing Immich and Google Photos settings are untouched.
3. Explicitly choose **JSON Feed** or **Existing + JSON Feed** independently for
   the Photos app, wallpaper and screen saver. All three default to existing
   Immich/Google sources. Mixed mode interleaves sources without shuffling the feed.
4. Change the **Refresh feed (toggle)** switch in either direction to discard its
   small cache and restart the ranked traversal with renewed media URLs.

The same fields are exposed through the shared settings API. Home Assistant gets
source selects, enable/refresh switches, a password-mode URL editor and a status
sensor. Long URLs (up to 4096 bytes) should be entered on-device or via the web UI;
the existing HA text transport only accepts 255 characters. Privileged Node settings
may control selection/enabling/refresh, but intentionally cannot read the URL.

## Feed shape

Only attachment URLs are used, never embedded bytes or HTML. Items need an ID.
The document needs the JSON Feed 1.1 version, a title and an items array.
Item order and attachment order are retained across absolute next_url pages.

```json
{
  "version": "https://jsonfeed.org/version/1.1",
  "title": "Ranked media",
  "next_url": "https://media.example.com/feed?page=2",
  "items": [
    {
      "id": "result-1",
      "attachments": [
        {"url": "https://cdn.example.com/one.jpg", "mime_type": "image/jpeg"}
      ]
    },
    {
      "id": "result-2",
      "attachments": [
        {"url": "https://cdn.example.com/two.mp4", "mime_type": "video/mp4"}
      ]
    }
  ]
}
```

Omit next_url on the last page. Relative URLs, userinfo, fragments and non-HTTP(S)
schemes are rejected. Refreshing a feed cannot renew a signed URL while its video
is already playing; return to Photos and refresh before reopening that item.

## Shared behavior and mixed media

Feed images use the same Photos frame/history, timing, fit (including slow pan),
transitions, wallpaper and screen-saver rendering as other providers.
Videos appear as navigable **Play video** cards; they do not auto-play or advance
on an image timer. Play opens the existing video player with its pause, seek,
volume and close controls. Closing returns to Photos. Wallpaper and screen saver
skip video cards and use images only. A video-only feed leaves those image surfaces
empty; it is not decoded as a still image.

Supported attachment declarations: image/jpeg, image/png, image/webp, video/mp4,
video/webm. Video container support is not a promise that every contained codec is
supported. Unsupported codecs and unavailable video links are reported by the
existing player. Refresh expired CDN URLs. Invalid/expired image links and unsupported
attachments are skipped; partial status asks the user to check MIME types and refresh.
Empty feeds have an empty status; unreachable and invalid documents have separate
actionable status. No raw feed error/URL is included in status or logs.

## Bounds and caching

A page is fetched on demand, not the entire result set: at most 2 MiB/page,
1024 items/page, 128 pages/traversal, 8192 examined attachments/traversal.
Pagination cycles stop with a partial status. At most one new page is requested per
five-second library tick, and at most 24 attachment descriptors are drawn per tick.
Image downloads have a 12 MiB cap, 15-second timeout, at most five redirects and an
8-megapixel/8192-pixel-axis decode ceiling. Images are then shrunk for the display.
The normal bounded prefetch/cache and 12-picture navigation history are reused;
videos are streamed/ranged by the existing player, not downloaded into this cache.
The feed has its own cache and cursor, so existing provider order is not disturbed.

Automatic refresh becomes eligible after 15 minutes but waits until the ranked
traversal is drained; a long ranked list is never cut short by that timer.
The final cache does not silently loop as though it were the whole feed.
A restart starts the feed from the first page; private remote video URLs are not
persisted in the photo index. Manual refresh starts over immediately on the next tick.
Fetches use the application's existing certificate/routing policy.
