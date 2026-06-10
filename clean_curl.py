#!/usr/bin/env python3
import sys
import re
import html
import datetime
import urllib.request
import urllib.error

def get_chrome_major_version():
	today = datetime.date.today()
	
	# Chrome transitions to a 2-week release cadence starting with version 153 on Sept 8, 2026
	sept_transition = datetime.date(2026, 9, 8)
	if today >= sept_transition:
		weeks_diff = (today - sept_transition).days // 14
		return 153 + weeks_diff
	
	# Pre-transition milestone tracking:
	# Chrome 152: Aug 25, 2026 to Sept 7, 2026
	# Chrome 151: July 28, 2026 to Aug 24, 2026
	# Chrome 150: June 30, 2026 to July 27, 2026
	# Chrome 149: June 2, 2026 to June 29, 2026
	if today >= datetime.date(2026, 8, 25):
		return 152
	elif today >= datetime.date(2026, 7, 28):
		return 151
	elif today >= datetime.date(2026, 6, 30):
		return 150
	elif today >= datetime.date(2026, 6, 2):
		return 149
	else:
		# Pre-June 2, 2026 (assuming standard 28-day cycle backwards)
		days_diff = (today - datetime.date(2026, 6, 2)).days
		return 149 + (days_diff // 28)

def fetch_url(url):
	chrome_major = get_chrome_major_version()
	user_agent = f"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/{chrome_major}.0.0.0 Safari/537.36"
	
	headers = {
		'User-Agent': user_agent,
		'Accept-Language': 'und'
	}
	
	req = urllib.request.Request(url, headers=headers)
	
	try:
		with urllib.request.urlopen(req, timeout=15) as response:
			return response.read().decode('utf-8', errors='ignore')
	except urllib.error.URLError as e:
		print(f"Error fetching URL: {e}", file=sys.stderr)
		sys.exit(1)

def clean_html(content):
	# 1. Remove script, style, xml, and title blocks (including their inner content)
	content = re.sub(r"(?is)<script[^>]*>.*?</script>", "", content)
	content = re.sub(r"(?is)<style[^>]*>.*?</style>", "", content)
	content = re.sub(r"(?is)<xml[^>]*>.*?</xml>", "", content)
	content = re.sub(r"(?is)<title[^>]*>.*?</title>", "", content)

	# 2. Convert breaks and paragraphs to newlines
	content = re.sub(r"(?i)<br[^>]*>", "\n", content)
	content = re.sub(r"(?i)</?p[^>]*>", "\n", content)

	# 3. Remove comments
	content = re.sub(r"<!--[\s\S]*?-->", "", content)

	# 4. Strip out ALL remaining HTML tags
	content = re.sub(r"<[^>]+>", "", content)

	# 5. Normalize whitespace and newlines
	content = re.sub(r"\s*(\r?\n)\s*", "\n", content)
	content = re.sub(r"[ \t\f\v]+", " ", content)

	return html.unescape(content).strip()

def main():
	if len(sys.argv) < 2:
		print("Usage: ./clean_curl.py <url>", file=sys.stderr)
		sys.exit(1)

	url = sys.argv[1]
	raw_html = fetch_url(url)
	cleaned_text = clean_html(raw_html)
	print(cleaned_text)

if __name__ == "__main__":
	main()