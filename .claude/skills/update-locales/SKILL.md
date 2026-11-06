---
name: update-locales
description: Update the bot's translations with gotext after adding or changing user-facing text (any loc.Sprintf or Locale.Sprintf string). Extracts new strings, fills in the translations, and rebuilds internal/translations/catalog.go. Use when strings changed, or when asked to run the locale or language update.
---

# Update the language files

This follows "Updating the language files" in `README.md`, plus what that section leaves out.
Run everything from the repo root, in the Bash tool.

## Which languages get translated

- **Maintained: `de-DE`, `fr-CA`, `en-US`.** These have a `messages.gotext.json`, which is what gets compiled into the catalog.
- **Not started: `es-PE`, `fr-FR`.** These only have an `out.gotext.json`, mostly empty. Leave them alone: `gotext` refreshes them, and they stay committed. Don't translate them or create a `messages` file unless asked.

If `locales/` no longer matches this list, go by which folders have a `messages.gotext.json`.

## 1. Set up the tools

```bash
source .claude/skills/update-locales/gotext-env.sh
```

This puts a matching `go` and a current `gotext` on the PATH for that one command, so **source it again at the start of every Bash call**. It builds `gotext` into a temp folder, and does not change `~/go/bin`, `go.mod` or `vendor`.

The script exists because an older `gotext` fails with errors like `undefined: min` or `couldn't load packages` when it can't parse the current Go standard library. If that happens with this script too, raise `GOTEXT_VERSION` in it.

## 2. Extract the strings

```bash
source .claude/skills/update-locales/gotext-env.sh
go generate internal/translations/translations.go
python -I .claude/skills/update-locales/report.py
```

`Missing entry` lines from `gotext` are normal at this stage, and there are always plenty for `es-PE` and `fr-FR`. Use the report instead:

- **Missing translation:** needs translating in step 3.
- **No longer used:** the string was deleted or its text changed.
- **Fuzzy:** `gotext` copied the English in. This is expected for `en-US`. In any other language, translate it and remove the `fuzzy` line.

## 3. Fill in the translations

Edit `out.gotext.json` in each maintained language, changing only the `translation` values.

- **Reuse before translating.** If a missing string pairs with a "no longer used" one, the text was reworded. When the change is only whitespace or punctuation, carry the old translation over and apply the same change. If the meaning changed, retranslate it.
- **Match the file.** Read the translations of neighbouring strings and copy their tone and word choices. For example, German mostly uses formal *Sie*, and `fr-CA` uses *vous*. Use the words already there for "event" and "attend".
- **Keep everything that isn't words:** `{Placeholders}`, HTML tags (written as `<b>` in these files), emoji, commands such as `/feed`, and `@furryplansbot`. Keep newlines where they are. `gotext` strips leading and trailing newlines from the `id`, so don't add them.
- **Say you translated it.** Machine translations should be reviewed by a speaker, so list them for the user at the end.

If you can't translate something with confidence, leave it empty, tell the user, and don't invent text. An empty string shows in English.

## 4. Promote the files and rebuild

```bash
source .claude/skills/update-locales/gotext-env.sh
for l in internal/translations/locales/*/; do
  if [ -f "$l/messages.gotext.json" ]; then mv -f "$l/out.gotext.json" "$l/messages.gotext.json"; fi
done
go generate internal/translations/translations.go
for l in internal/translations/locales/*/; do
  if [ -f "$l/messages.gotext.json" ]; then rm -f "$l/out.gotext.json"; fi
done
```

The `out` files are removed only from folders with a `messages` file, so the `es-PE` and `fr-FR` ones stay.

## 5. Check the result

```bash
source .claude/skills/update-locales/gotext-env.sh
python -I .claude/skills/update-locales/report.py
go test ./...
git status --short -- internal/translations
```

- The report should show nothing missing for the maintained languages. If it does, something didn't get filled in or wasn't promoted.
- Look through `git diff` on the `messages` files. Only the strings you meant to change should differ. `gotext` writes `<`, `>` and `&` as `<`, `>` and `&`, and may reorder entries. That's only formatting.
- Tell the user what was translated, what was reused, and anything left empty.

Don't test against live Telegram. The user runs all live tests.
