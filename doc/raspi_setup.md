# Raspberry Pi display setup

This is the full setup for the `CloudToRaspi` output path: an SPI TFT
display attached to a Raspberry Pi, driven by the framebuffer (`fbtft`),
fed by `raspi/bin/pipetodisp.sh` + `raspi/bin/fb-image-watcher.sh` /
`raspi/services/fb-image-watcher.service`. The CUPS-backend side
(`raspi/scripts/toraspi.sh`, run on the machine with the backend, not on
the Pi) is already covered by the main
[README](../README.md#output-on-an-external-display-optional) - this guide
is specifically the Raspberry Pi side, since getting a cheap SPI TFT HAT
correctly recognized and wired took real trial and error the first time.

**Do the hardware verification steps (1-2) before installing any of the
software below** - they catch wiring problems independently of the
scripts, and are much faster to redo than debugging the whole pipeline.

## 1) Display driver (`/boot/firmware/config.txt`)

There are many different cheap SPI TFT HATs, each needing its own
`dtoverlay`, and this project has been run against more than one. Keep
the line(s) for hardware you actually use and comment out the rest -
don't delete a working config just because a different display is now
the primary one, the old hardware tends to come back:

```
# ILI9486 3.5" HAT, "tft35a" overlay (colon syntax) - verified working:
dtoverlay=tft35a:rotate=90

# Waveshare 3.5" (A) HAT / clone, "waveshare35a" overlay (comma syntax) -
# verified working:
#dtoverlay=waveshare35a,rotate=270
```

Only one `dtoverlay=...` line for the display should be active (uncommented)
at a time. Check your display's own documentation for the correct overlay
name if neither matches (`ls /boot/firmware/overlays/ | grep -i wave` /
`grep -i tft` lists what's available on this OS image).

**No spaces around the commas** in the comma-style syntax -
`dtoverlay=waveshare35a, rotate=270` (with a space) loads the base
overlay fine but silently fails to apply `rotate` at all, with no error
anywhere (not in `dmesg`, not from `dtoverlay`) - the display just looks
exactly like it would without the parameter, which is easy to misread as
"rotate doesn't work on this overlay".

`rotate` takes 0/90/180/270; if the image ends up upside down, it's
usually off by exactly 180 from whatever you have. List the parameters a
specific overlay actually supports with `dtoverlay -h <name>` (e.g.
`waveshare35a`, not `wave35` - the overlay name must match exactly).

Reboot after changing `config.txt`:

```bash
sudo reboot
```

### If colors are inverted (a solid black image shows up white)

Some of these panels invert every color (black ↔ white, and everything
else) regardless of what's actually written to the framebuffer, with no
`dtoverlay` parameter to fix it (the documented `bgr` parameter swaps
red/blue hues, not a full inversion - worth a quick try, but don't expect
it to fix this specific symptom). Confirm it's a clean, uniform inversion
rather than something else by negating a solid reference image and
checking it comes out correct:

```bash
convert -size 480x320 xc:black /tmp/black.png
convert -negate /tmp/black.png /tmp/notblack.png
cat /tmp/notblack.png | pipetodisp.sh 10
```

If that shows solid black (correct), set `INVERT=1` on the
`fb-image-watcher` service (see step 3) - it negates every image in
software before displaying it, which cancels the panel's own inversion
back out.

## 2) Verify the display works, before installing anything

Check that the driver loaded and find out **which** `/dev/fb*` is the
actual display:

```bash
sudo dmesg | grep -i -E "fb_ili9486|vc4-drm"
```

A clean-looking driver message (resolution, `fps=...`, SPI speed) is
**not proof the display is actually wired correctly** - this class of
display is write-only over SPI (no readback), so the kernel driver
registers successfully even if the data/control pins are wrong. Also,
which fb number the SPI TFT gets vs. the Pi's own HDMI/DRM output
(`vc4drmfb`) can vary by probe timing - don't assume `/dev/fb1` is correct
just because another Pi used that number; read it from the `dmesg` output
above for *this* boot.

With the right device identified, write raw noise directly to it - this
bypasses all of this project's software and tests the display/wiring in
isolation:

```bash
sudo bash -c 'for i in $(seq 1 20); do head -c 307200 /dev/urandom > /dev/fbN; sleep 0.3; done'
```

(`307200` = 480×320×2 bytes, adjust if your display's resolution differs;
replace `fbN` with the device found above.) You should see colorful
static/noise on the screen immediately. If you see **nothing at all**,
stop here and fix the hardware before continuing - installing the full
software stack won't help:

- Check the backlight: does the panel show *any* light (even without an
  image), or is it completely dark? A dark backlight points to a
  power/backlight wiring issue rather than a data-line problem.
- Recheck every pin against the display's own pinout diagram, not just
  the four SPI data lines (MOSI/SCK/CS) - these HATs also need RESET,
  DC/A0, and power (VCC/GND), and a backlight pin on top of that. A
  connector that's just one GPIO row offset will often still produce a
  "healthy" `dmesg` and zero display output, for the reason above.

## 3) Install the display pipeline

Packages:

```bash
sudo apt install fbi imagemagick inotify-tools
```

Copy the scripts from the repo (run from a checkout, or `scp` them over):

```bash
scp raspi/bin/* <pi-host>:/tmp/
ssh <pi-host> 'sudo cp /tmp/pipetodisp.sh /tmp/fb-image-watcher.sh /usr/local/bin/ && sudo chmod +x /usr/local/bin/pipetodisp.sh /usr/local/bin/fb-image-watcher.sh'
```

On the Pi, create the watch directory (default `~/fb-drop`, must match
the user that files get `scp`'d in as - see `raspi/services/fb-image-watcher.service`):

```bash
mkdir -p ~/fb-drop
```

Install the systemd service:

```bash
scp raspi/services/fb-image-watcher.service <pi-host>:/tmp/
ssh <pi-host> sudo cp /tmp/fb-image-watcher.service /etc/systemd/system/
```

**Before enabling it**, edit `/etc/systemd/system/fb-image-watcher.service`
on the Pi:

- `WATCH_DIR` - must point at the home directory of the user files are
  delivered to (the placeholder is `/home/CHANGE_ME/fb-drop`).
- `FB_DEVICE` - must match the device found in step 2 (default
  `/dev/fb1`; change it if your Pi registered the TFT as `/dev/fb0`
  instead).
- `INVERT` - uncomment and set to `1` if your panel shows inverted colors
  (see step 1).

Then:

```bash
ssh <pi-host> 'sudo systemctl daemon-reload && sudo systemctl enable --now fb-image-watcher'
ssh <pi-host> systemctl status fb-image-watcher --no-pager
```

## 4) End-to-end test

From the repo, with a sample image:

```bash
scp doc/images/output_on_raspberry.jpg <pi-host>:~/fb-drop/
```

The image should appear on the display for `TIMEOUT` seconds (default 60,
set via the service's `TIMEOUT` environment variable) and then clear back
to black. Equivalently, from the machine running the CUPS backend:

```bash
ssh <pi-host> pipetodisp.sh 10 < path/to/some.pdf
```

(`pipetodisp.sh` accepts stdin directly, same as what `toraspi.sh` sends
over `ssh` in production - see the README section linked at the top for
hooking this up to the actual backend via `RASPI_HOST`,
`RASPI_DISPLAY_TIMEOUT`, and `MYSHARE_DISPLAY_HOOK` in `backend.env`.)

## Troubleshooting

If images don't show up once everything is "installed", work top-down,
not bottom-up - start at step 2's raw noise test again, since a
reproducible "clean dmesg, nothing on screen" result is a hardware issue,
not a bug to chase in `fb-image-watcher.sh`:

1. Re-run the raw noise test from step 2 on the exact device set in the
   service's `FB_DEVICE`. If that alone doesn't show anything, it's
   wiring/backlight/power, not the software pipeline below.
2. `systemctl status fb-image-watcher` - confirm it's `active (running)`,
   not crash-looping.
3. `tail -f /var/log/fb-image-watcher.log` while copying a test image in
   - repeated font-loading lines from `fbi` are normal; errors are not.
4. Confirm the file actually arrives in `WATCH_DIR` as the same user the
   service watches (a mismatch between the `scp`/`ssh` user and the
   service's configured `WATCH_DIR` means the file lands somewhere the
   watcher never sees).
5. Images show up but colors look wrong (not missing, just inverted) -
   see "If colors are inverted" in step 1 and the service's `INVERT`
   setting, not a pipeline bug.
6. A specific image shows up as a solid black rectangle while others work
   fine - that image likely has a transparent background (`identify
   -verbose <file> | grep Alpha`); `fb-image-watcher.sh` flattens
   transparency onto white before displaying, but if you're testing with
   an older copy of the script it may still be missing that step.
7. After editing `/etc/systemd/system/fb-image-watcher.service` by hand
   (e.g. to add `INVERT`), double-check `WATCH_DIR` is still your real
   home directory and not the `/home/CHANGE_ME/fb-drop` placeholder -
   re-copying the service file from the repo to pick up one setting
   silently overwrites any other customization already made on the Pi.

<!-- Add your own notes/additions from practice here, e.g. which overlay
     name worked for your specific display model. -->
