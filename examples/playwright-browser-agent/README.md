# jeq browser example

`hn-browser.sh` is a direct CLI composition: `playwright-cli` observes the page, `jq` builds one finite Choice request, and `jeq ask --request -` predicts the next opaque candidate. The shell script validates safe same-host navigation, bounds every step and payload, and independently verifies the final date, discussion count, and bounded comments.

Run the offline acceptance test:

```sh
./test-hn.sh
```

A paid headed run requires explicit authorization and configured credentials:

```sh
JEQ_GOAL='navigate to yesterday then the most-discussed discussion' ./hn-browser.sh
```
