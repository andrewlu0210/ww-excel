#!/bin/bash
# Builds "WW Excel.app" and a DMG in dist/, optionally signed with a
# Developer ID and notarized.
#
#   scripts/package.sh                 # ad-hoc signed, for local testing
#   VERSION=1.0.1 scripts/package.sh   # version (default: Info.plist)
#   UNIVERSAL=1 scripts/package.sh     # arm64 + amd64
#
# Distribution (Developer ID + notarization):
#   SIGN_IDENTITY="Developer ID Application: Name (TEAMID)" \
#   NOTARY_PROFILE=ww-notary scripts/package.sh
#
# Create the notary profile once with:
#   xcrun notarytool store-credentials ww-notary --apple-id you@example.com --team-id TEAMID
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$HOME/go/bin:$PATH"

PLIST=build/darwin/Info.plist
VERSION=${VERSION:-$(/usr/libexec/PlistBuddy -c "Print :CFBundleShortVersionString" "$PLIST")}
SIGN_IDENTITY=${SIGN_IDENTITY:--}
NOTARY_PROFILE=${NOTARY_PROFILE:-}
APP_NAME="WW Excel"
OUT=dist
APP="$OUT/$APP_NAME.app"
DMG="$OUT/WW-Excel-$VERSION.dmg"

if [ -n "$NOTARY_PROFILE" ] && [ "$SIGN_IDENTITY" = "-" ]; then
  echo "NOTARY_PROFILE needs SIGN_IDENTITY (a Developer ID Application certificate)" >&2
  exit 1
fi

echo "==> WW Excel $VERSION"
echo "==> Icons"
wails3 task common:generate:icons

echo "==> Frontend"
wails3 generate bindings -clean=true -ts -i >/dev/null
(cd frontend && npm run build >/dev/null)

export CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=12.0
export CGO_CFLAGS="-mmacosx-version-min=12.0" CGO_LDFLAGS="-mmacosx-version-min=12.0"
LDFLAGS="-s -w -X main.version=$VERSION"
build() { # arch output
  GOOS=darwin GOARCH=$1 go build -tags production -trimpath -buildvcs=false -ldflags "$LDFLAGS" -o "$2" .
}

echo "==> Go"
rm -rf "$OUT" && mkdir -p "$OUT"
if [ "${UNIVERSAL:-}" = "1" ]; then
  build arm64 "$OUT/ww-excel-arm64"
  build amd64 "$OUT/ww-excel-amd64"
  lipo -create -output "$OUT/ww-excel" "$OUT/ww-excel-arm64" "$OUT/ww-excel-amd64"
  rm "$OUT/ww-excel-arm64" "$OUT/ww-excel-amd64"
else
  build "$(go env GOARCH)" "$OUT/ww-excel"
fi

echo "==> App bundle"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
mv "$OUT/ww-excel" "$APP/Contents/MacOS/ww-excel"
cp build/darwin/icons.icns "$APP/Contents/Resources/"
[ -f build/darwin/Assets.car ] && cp build/darwin/Assets.car "$APP/Contents/Resources/"
cp "$PLIST" "$APP/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $VERSION" -c "Set :CFBundleVersion $VERSION" \
  -c "Set :CFBundleGetInfoString WW Excel $VERSION" "$APP/Contents/Info.plist"
xattr -cr "$APP"

echo "==> Sign ($SIGN_IDENTITY)"
if [ "$SIGN_IDENTITY" = "-" ]; then
  codesign --force --deep --sign - "$APP"
else
  # Hardened runtime and a secure timestamp are required for notarization.
  codesign --force --deep --options runtime --timestamp \
    --entitlements build/darwin/entitlements.plist --sign "$SIGN_IDENTITY" "$APP"
fi
codesign --verify --deep --strict "$APP"

echo "==> DMG"
STAGE=$(mktemp -d)
cp -R "$APP" "$STAGE/"
ln -s /Applications "$STAGE/Applications"
hdiutil create -volname "$APP_NAME $VERSION" -srcfolder "$STAGE" -fs HFS+ -format UDZO -ov "$DMG" >/dev/null
rm -rf "$STAGE"
if [ "$SIGN_IDENTITY" != "-" ]; then
  codesign --force --timestamp --sign "$SIGN_IDENTITY" "$DMG"
fi

if [ -n "$NOTARY_PROFILE" ]; then
  echo "==> Notarize"
  xcrun notarytool submit "$DMG" --keychain-profile "$NOTARY_PROFILE" --wait
  xcrun stapler staple "$DMG"
  xcrun stapler validate "$DMG"
  spctl --assess --type open --context context:primary-signature -v "$DMG"
fi

shasum -a 256 "$DMG" | tee "$DMG.sha256"
echo "==> Done: $DMG"
