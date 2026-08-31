---
title: Themes
description: Every theme the sqly shell draws SQL in, shown on the same statement.
weight: 45
---

sqly colors SQL as you type it. `.theme` shows the theme in effect and the ones
available, and takes one to switch to:

```text
sqly:~/data(table)$ .theme dracula
theme set to dracula
```

The choice is remembered, so it is made once rather than every time. The
[shell page](/shell/#colors) has the details; this page is the pictures.

Each screenshot below is the same statement, chosen to hold one of everything
the highlighter names: a keyword, a column, a table, a string literal, a number,
and a comment. `actor` is both a column of the file and the table it was read
from, so the two colors for a name are visible side by side.

```sql
SELECT actor, gross AS 'top' FROM actor WHERE gross > 42 -- a comment
```

sqly colors the text; your terminal supplies the background. A theme meant for a
light terminal is shown on one below, and will read badly on a dark one.

## night-owl

The default, and the colors sqly has always drawn its prompt in.

![the night-owl theme](/img/themes/night-owl.png)

## dracula

![the dracula theme](/img/themes/dracula.png)

## monokai

![the monokai theme](/img/themes/monokai.png)

## nord

![the nord theme](/img/themes/nord.png)

## solarized

![the solarized theme](/img/themes/solarized.png)

## gruvbox

![the gruvbox theme](/img/themes/gruvbox.png)

## tokyo-night

![the tokyo-night theme](/img/themes/tokyo-night.png)

## catppuccin

![the catppuccin theme](/img/themes/catppuccin.png)

## vscode

![the vscode theme](/img/themes/vscode.png)

## github-light

For a light terminal, and shown on one.

![the github-light theme](/img/themes/github-light.png)

## accessible

High contrast, for a terminal where the others are hard to read.

![the accessible theme](/img/themes/accessible.png)

## none

No coloring at all, for a terminal or a reader that wants the input plain. It is
remembered like any other choice.

![the none theme](/img/themes/none.png)
