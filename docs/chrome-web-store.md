# Releasing the extension to the Chrome Web Store

The `extension` workflow (`.github/workflows/extension.yml`) publishes `extension/` to the Chrome Web
Store. It authenticates to Google without keys, through Workload Identity Federation: GitHub's OIDC
token for this repository impersonates a Google Cloud service account.

## Releasing a version

1. Bump `version` in `extension/public/manifest.json` and `extension/package.json` (they must match),
   for example with `cd extension && npm version X.Y.Z --no-git-tag-version` plus the manifest edit.
2. Commit and push to `main`.
3. Tag and push: `git tag ext-vX.Y.Z && git push origin ext-vX.Y.Z`.

The workflow checks that the tag matches the manifest, runs lint, typecheck and tests, builds,
zips `extension/dist` (also kept as the `kala-extension` run artifact), uploads the ZIP and
submits it for review. The version goes live once Google approves it, which can take a few days.

- Don't release while a previous version is still in review. The submit step fails until that
  review is over.
- To retry a release without a new tag, run the workflow by hand on `main`
  (`gh workflow run extension.yml --ref main`). The tag check is skipped, and the version in the
  manifest is uploaded. Uploading the same version to an unpublished draft again is accepted.
- If you change the item's visibility in the dashboard, publish once by hand before the API can
  publish again.

## One-time setup

### 1. Create the store item

The API cannot create an item, so the first upload is manual.

1. Build the ZIP. `manifest.json` must sit at its root:
   `make ext && (cd extension/dist && zip -r ../kala-extension.zip .)`
2. Upload it in the [Developer Dashboard](https://chrome.google.com/webstore/devconsole) with **New item**.
3. In publisher **Settings**, enter a contact email and verify it. Without a verified email, the
   publisher can't submit anything; the API answers `MISSING_CONTACT_EMAIL` / `UNVERIFIED_CONTACT_EMAIL`.
4. Fill in the item's tabs. Submitting fails with `INVALID_ARGUMENT` ("does not meet the
   requirements") until the dashboard lists nothing as missing.
   - **Store listing:** description, category, language, and at least one 1280×800 or 640×400 screenshot.
     The 128×128 icon comes from the package (`extension/public/icons/icon-128.png`).
   - **Privacy:** see [Privacy tab](#privacy-tab) below.
   - **Distribution:** visibility and regions.
5. Note the **item ID** (32 letters, shown on the item) and the **publisher ID** (publisher
   **Settings**).

### 2. Google Cloud

Use a dedicated project. Billing isn't needed. Log in first with `gcloud auth login`.

```sh
P=kala-cws-XXXX; REPO=mdallastella/kala   # project IDs are global: pick a free one
gcloud projects create $P
gcloud services enable chromewebstore.googleapis.com iamcredentials.googleapis.com --project $P
gcloud iam service-accounts create cws-publisher --project $P
gcloud iam workload-identity-pools create github --location global --project $P
gcloud iam workload-identity-pools providers create-oidc github --project $P \
  --location global --workload-identity-pool github \
  --issuer-uri https://token.actions.githubusercontent.com \
  --attribute-mapping google.subject=assertion.sub,attribute.repository=assertion.repository \
  --attribute-condition "assertion.repository == '$REPO'"
POOL=$(gcloud iam workload-identity-pools describe github --location global --project $P --format 'value(name)')
gcloud iam service-accounts add-iam-policy-binding cws-publisher@$P.iam.gserviceaccount.com --project $P \
  --role roles/iam.workloadIdentityUser --member "principalSet://iam.googleapis.com/$POOL/attribute.repository/$REPO"
gcloud iam workload-identity-pools providers describe github --location global \
  --workload-identity-pool github --project $P --format 'value(name)'   # → GCP_WIF_PROVIDER
```

### 3. Give the service account access to the store

In the Developer Dashboard, go to publisher **Settings → Service accounts** and add
`cws-publisher@<project>.iam.gserviceaccount.com`. It can take a few minutes to start working.
Until then the API answers 403.

### 4. Repository variables

These are variables, not secrets: the workflow reads `vars.*`.

```sh
gh variable set CWS_PUBLISHER_ID --body <publisher id>
gh variable set CWS_ITEM_ID --body <item id>
gh variable set GCP_WIF_PROVIDER --body projects/<number>/locations/global/workloadIdentityPools/github/providers/github
gh variable set GCP_SERVICE_ACCOUNT --body cws-publisher@<project>.iam.gserviceaccount.com
```

## Privacy tab

The answers that were submitted for 0.1.1. Update them if the extension's permissions or data
handling change.

**Single purpose**

> Quickly add a task to the user's own kala task manager from any page, optionally starting a
> time-tracking timer on it. The popup sends the task to the kala server the user configured, through
> its REST API with a personal access token, and the toolbar icon shows a dot while a timer is running.

**storage**

> Saves the extension's two settings: the address of the user's kala server and the personal access
> token used to call its API (chrome.storage.local). Also caches, for the browser session only, the
> user's tag list and timezone from that server (chrome.storage.session), so the quick-add box can
> suggest tags and parse due dates without fetching them every time the popup opens. Nothing is sent
> anywhere except the user's own kala server.

**alarms**

> A one-minute alarm checks the user's kala server for a running timer and shows or hides a red dot on
> the toolbar icon. Timers can be started or stopped from the kala web app or other devices, and the
> Manifest V3 service worker cannot keep its own interval, so the alarm keeps the indicator accurate.

**Host permissions** (`optional_host_permissions`): requested at run time for one origin only, the
kala server URL entered in the options page, to call its API. There are no content scripts, so the
extension never reads the pages the user visits.

**Remote code:** No.

**Data usage:** only *Authentication information* is collected (the access token, kept in the browser
and sent only to the user's server). All three certifications are ticked.

**Privacy policy URL:** <https://github.com/mdallastella/kala/blob/main/extension/PRIVACY.md>

## Troubleshooting

| Symptom | Cause |
|---|---|
| `auth` step: must specify `workload_identity_provider` | Repository variables not set |
| `auth` step: permission or attribute-condition error | Repository name doesn't match the provider's condition, or the IAM binding is missing |
| Upload 403 | Service account not added in the dashboard, or not propagated yet |
| Publish 400, `MISSING_CONTACT_EMAIL` / `UNVERIFIED_CONTACT_EMAIL` | Publisher contact email missing or unverified |
| Publish 400, `INVALID_ARGUMENT` | Store listing, Privacy or Distribution incomplete. The dashboard's submit button lists what's missing |
| `zip` writes the archive in the wrong place | An environment variable named `ZIP` is treated by Info-ZIP as default options. The workflow uses `ARCHIVE` |
