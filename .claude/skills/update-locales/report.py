"""Reports what still needs doing in each maintained language. Run from the repo root."""
import glob
import json
import os
import sys

sys.stdout.reconfigure(encoding="utf-8")


def key(message):
    # An id is a list when a message has several source forms.
    value = message["id"]
    return json.dumps(value, ensure_ascii=False) if isinstance(value, list) else value


def load(path):
    with open(path, encoding="utf-8") as f:
        return {key(m): m for m in json.load(f).get("messages", [])}


def show(title, items):
    if items:
        print(f"  {title}: {len(items)}")
        for item in items:
            print("    " + repr(item)[:200])


for folder in sorted(glob.glob("internal/translations/locales/*/")):
    lang = os.path.basename(folder.rstrip("/\\"))
    messages_path = os.path.join(folder, "messages.gotext.json")
    out_path = os.path.join(folder, "out.gotext.json")
    if not os.path.exists(messages_path):
        print(f"{lang}: not started, skipped")
        continue

    messages = load(messages_path)
    # After extraction the out file is the newest; after the final rebuild only messages remains.
    current = load(out_path) if os.path.exists(out_path) else messages
    print(f"{lang}: {len(current)} strings")
    show("Missing translation", [i for i, m in current.items() if not m.get("translation")])
    show("No longer used", [i for i in messages if i not in current])
    fuzzy = [i for i, m in current.items() if m.get("fuzzy")]
    if lang.startswith("en"):
        print(f"  Fuzzy: {len(fuzzy)} (expected for English)")
    else:
        show("Fuzzy", fuzzy)
