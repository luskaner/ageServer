# Menu backgrounds (AoE IV)

The AoE IV client shows the `frontEndImagePath` of a type-0 community event covering
the current time as the main-menu background. The official service publishes one such
event per live promotion; this server instead publishes the periods an administrator
configures, for **every player** on the server.

## The catalogue

`resources/responses/age4/menuBackgrounds.json` lists the images the client can display:

```json
[
  0,
  [
    {"id":129,"name":"Raiders of the North","frontEndImagePath":"\\Images\\backgrounds\\events\\xp7_keyart.png","sourceEventId":129,"tags":["raiders-of-the-north","viking"]}
  ]
]
```

It ships 30 entries, ids `100`–`129`, and is a **subset** of what the official service
knows: the official's current period uses an id that is not in this file. Entries
without a positive `id`, a `name` or a `frontEndImagePath` are skipped. The file is a
captured asset, like `communityEvents.json`, so there is no signature to verify.

## The file

The schedule is `resources/config/age4/menuBackgrounds.toml`. It is **not shipped**, so
the feature is off by default and every player keeps the background the game shows
today. Like `news.toml`, it is read when the server starts: restart the server after
editing it.

## Fields

Each `[[background]]` table is one period: the image shown from `start` until `end`.
Outside every window the game shows its own background, so **a gap is how you say "no
background"**.

| Field   | Required | Meaning |
|---------|----------|---------|
| `image` | yes      | Catalogue id; resolves to the image path the client loads. |
| `start` | yes      | Absolute RFC 3339 instant with an offset or `Z`. The window includes `start`. |
| `end`   | yes      | Absolute RFC 3339 instant. The window excludes `end`. |
| `event` | no       | Event id for this period. Defaults to `900000 + image`. |
| `name`  | no       | Event name. Defaults to the catalogue entry's name. |

```toml
[[background]]
image = 129                                    # Raiders of the North
start = 2026-11-01T17:00:00Z
end   = 2026-11-15T17:00:00Z

[[background]]
image = 125                                    # Season's Feast
start = 2026-12-05T18:00:00Z
end   = 2027-02-05T18:00:00Z
```

`event` and `name` exist to mirror the official service exactly: it gives each period
its own event id and a period-specific name, and reuses images across periods. Because
the default id is `900000 + image`, showing **the same image in two periods** needs an
explicit `event` on at least one of them; the error tells you so.

## What gets published

The endpoint serves the **history** of the schedule, the way the official service does:

1. every period whose `start` has passed is published, including periods that already
   ended;
2. a period that has not started yet is never published;
3. periods covering the current time come first, ascending by event id, followed by the
   rest, ascending by event id;
4. every event has the official 8 fields, an empty description, `customdata` that is
   exactly `{"frontEndImagePath":"..."}`, and the far-future expiry the official service
   also uses;
5. the event id is `event` when set, otherwise `900000 + image`.

An ended period keeps being published until it is removed from the file, so the file is
also the bound on the history the server serves.

## Validation

The server **refuses to start** when the file is invalid, naming the file and the
offending entry. This is deliberate, following `news.toml`: the catalogue is bundled
and static, so a bad value cannot become correct later, and it is reported before any
listener binds.

| Rule | Why |
|------|-----|
| `image` exists in the catalogue | otherwise the client is handed a path it may not own |
| `start` and `end` are present, `start < end` | a missing date cannot define a window |
| no two windows overlap | overlap makes "which one" undefined; the official has none |
| event ids are unique | no official response repeats one |
| every key and table name is a known one | a typo would otherwise switch the feature off silently |
| an explicit `event` is below `1000` or at least `900000` | the ids from `1000` upwards are the ranked season range; a background landing there would be read as a season and break ranked play |
| the file parses as TOML | same as `news.toml` |

An absent or empty file is not an error: it means the feature is off. If the file exists
for a title that ships no `menuBackgrounds.json` catalogue the server logs a warning and
shows nothing, rather than failing.

## Interaction with ranked

Background events are community-event type `0`, and their default ids sit in the
reserved `900000+` range, disjoint from the ranked season ids this server generates
(`1000+`). They are prepended to the event list and never replace the type-6 season
events, so ranked play is unaffected. An explicit `event` must therefore stay below
`1000` (where every id the official service uses lives) or at `900000` or above; the
server refuses to start otherwise.

One deliberate difference from the official service: it sorts the whole table with the
active events first, so its background sits among the season events. This server leaves
the season events in the order it already emits them and puts the background events in
front of them. The client selects by window, so the result is the same.

## A worked year

The real dates come from the official service; the ids below are catalogue entries
that represent the same images, so the file passes validation as-is:

```toml
[[background]]
event = 124
name  = "Map Monsters: Summer Sights"
image = 124
start = 2025-04-29T17:00:00Z
end   = 2025-06-10T17:00:00Z

[[background]]
event = 127
name  = "Conquest"
image = 127
start = 2025-06-11T17:00:00Z
end   = 2025-06-25T17:00:00Z

[[background]]
event = 133
name  = "Rise to Ruin"
image = 128
start = 2025-06-26T17:00:00Z
end   = 2025-07-29T17:00:00Z

[[background]]
event = 136
name  = "Hallows Hearth"
image = 118
start = 2025-10-02T17:00:00Z
end   = 2025-10-29T18:00:00Z

[[background]]
event = 149
name  = "Season 12 - Seasons Feast - Key Art - 2025"
image = 125
start = 2025-12-05T18:00:00Z
end   = 2026-02-05T18:00:00Z

[[background]]
event = 290
name  = "Trade Secrets - Challenges 13"
image = 129
start = 2026-11-01T17:00:00Z
end   = 2026-11-15T17:00:00Z
```

Every `image` above exists in the shipped catalogue; every other id is invented for
the example. Do not copy the ids from the official service: an unknown `image` stops
the server from starting.
