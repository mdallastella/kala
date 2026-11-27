# kala Chrome extension: privacy policy

The kala extension is a client for a kala server that you run or choose yourself. It has no
server, analytics, or tracking of its own.

## What it stores

- **The address of your kala server and your personal access token**, saved in Chrome's local
  extension storage (`chrome.storage.local`) on your device.
- **Your tag list and timezone**, fetched from your kala server and cached for the current browser
  session only (`chrome.storage.session`), to suggest tags and interpret due dates.

## What it sends, and where

Everything goes to the kala server address you entered, and nowhere else. That covers the tasks
you add, timer start and stop requests, and periodic checks for a running timer. Each request is
authenticated with your access token. The extension asks Chrome for permission to reach that one
address only.

## What it does not do

- It does not read the content of the pages you visit, or your browsing history.
- It does not sell or transfer data to third parties, or use it for anything other than adding
  tasks and tracking time in your kala server.
- It does not load or run remote code.

## Removing your data

Removing the extension deletes everything it stored. Data on your kala server is governed by
whoever runs that server.

## Contact

Open an issue at https://github.com/mdallastella/kala/issues.
