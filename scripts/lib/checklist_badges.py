# SPDX-License-Identifier: AGPL-3.0-or-later
#
# The badge rule for docs/FOX_Checklist.html (audit v2 G31): a row badged done or
# done-by-hand must carry nothing unfinished. Prints the ids that break it.
#
# "lim" (a limit of what it does) and "next" (a planned extension) do not count
# against a row: neither makes its own claim untrue. How well something was tested
# belongs in the "Tested by" column.
import re, sys
h = open(sys.argv[1]).read()
bad = [i for i, st, body in re.findall(r'<tr data-id="([A-Z0-9]+)" data-st="(done|man)">(.*?)</tr>', h, re.S)
       if re.search(r'<li class="(part|todo|bad)"', body)]
print(" ".join(bad))
