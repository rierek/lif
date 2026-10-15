### Parent values

At a `--parent` value the picker offers only items that can be the parent: the type rules of `--parent` completion apply, the edited item and its descendants are left out, and so is every item whose status is an archive status (by default `completed` and `scrapped`; a `statuses:` entry with `archive: false` keeps a status in). Typing an archived item's ID after `--parent` is still accepted.
