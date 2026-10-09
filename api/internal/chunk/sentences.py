"""Offset-only pySBD bridge. No cleaning: source text and rune anchors stay exact."""
import json
import sys

import pysbd

segmenter = pysbd.Segmenter(language="en", clean=False, char_span=True)
result = []
for text in json.load(sys.stdin):
    spans = []
    for span in segmenter.segment(text):
        start, end = span.start, span.end
        while start < end and text[start].isspace():
            start += 1
        while end > start and text[end - 1].isspace():
            end -= 1
        if start < end:
            # pySBD also emits standalone headings. Keep a heading with its
            # following prose rather than ending a reading card on "CONTENTS".
            if spans and text[spans[-1]["Start"]:spans[-1]["End"]].rstrip('\"\'”’)]}').rstrip()[-1:] not in ('.', '!', '?', '。', '！', '？'):
                spans[-1]["End"] = end
            else:
                spans.append({"Start": start, "End": end})
    result.append(spans)
json.dump(result, sys.stdout)
