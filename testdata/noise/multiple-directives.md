---
id: SHOULD-BE-IGNORED
title: "This has an ID but yat:ignore should win"
status: todo
type: Task
priority: High
dependencies: []
yat: "ignore, experimental"
---

## Description

This file has all the fields of a valid item, but the yat:ignore
directive should cause it to be completely skipped.
