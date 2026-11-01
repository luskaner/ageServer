# News

The `/game/news/getNews` endpoint returns the news configured for each game. They are read from
`resources/config/<game>/news.toml`, where `<game>` is one of `age1`, `age2`, `age3`, `age4` or `athens`.

## Adding news

Each news item is a TOML table whose key is the news id. The file may contain zero, one or more items:

```toml
[1]
category = "category"
title = "title"
subtitle = "subtitle"
text = "text"
image_url = "https://example.com/image.jpg"
body = "This is the body of the news article."
metadata = "{ \"version\": 1, \"categories\": [\"popup\", \"hero\", \"featured\", \"widget\"], \"showAlways\": false, \"url\": \"https://aoe.ms/AoEIVRaidersOfTheNorthAvailableNow\" }"
date_visible = "1793294400"
date_expiry = "1798480800"

[2]
category = "update"
title = "Another title"
subtitle = "Another subtitle"
text = "Another text"
image_url = "https://example.com/another.jpg"
body = "Another body."
metadata = "{}"
date_visible = "1793294400"
date_expiry = "1798480800"
```

The table key (`[1]`, `[2]`, ...) is the news `id` sent to the game. Items are returned in ascending numeric id order.
The file is loaded when the server starts, so restart the server after adding or editing news.

### Fields

| Field          | Type   | Description                                                                 |
|----------------|--------|-----------------------------------------------------------------------------|
| `category`     | string | News category.                                                              |
| `title`        | string | News title.                                                                 |
| `subtitle`     | string | News subtitle.                                                              |
| `text`         | string | Short news text.                                                            |
| `image_url`    | string | URL of the news image.                                                      |
| `body`         | string | Full news body.                                                             |
| `metadata`     | string | JSON document, escaped as a TOML string, with the client's news metadata.   |
| `date_visible` | string | Unix timestamp (seconds) from which the news is visible.                    |
| `date_expiry`  | string | Unix timestamp (seconds) at which the news expires.                         |

Every field is sent as received: dates and metadata stay strings.

## Response

The endpoint returns the usual result envelope with the news list as its second element and an empty third element:

```json
[
  0,
  [
    [
      1,
      "category",
      "title",
      "subtitle",
      "text",
      "https://example.com/image.jpg",
      "This is the body of the news article.",
      "{ \"version\": 1 }",
      "1793294400",
      "1798480800"
    ]
  ],
  []
]
```

## No news

If a game has no `news.toml`, the endpoint returns an empty list:

```json
[0, [], []]
```

An empty file behaves the same way. A malformed `news.toml` is a configuration error and stops the server at startup
instead of silently serving an empty list.
