# Fixture wiki document

A lede paragraph stating what this document is, so the shell contract has a
title and a lede to extract and the home card has a subtitle.

## Why this tree exists

This tree is a whole, well-formed knowledge tree in miniature: two layers, a
declared artifact, per-doc options, and a changelog opt-in. The contract tests
read it when no consumer tree is named, so the loaders stay covered in a repo
that ships no consumer of its own.

## What it must keep

Every document here satisfies the authoring contract: one H1 on line 1, a lede
paragraph beneath it, and no second H1 outside a fence. A change that breaks
those invariants should fail the loader tests, which is the point.
