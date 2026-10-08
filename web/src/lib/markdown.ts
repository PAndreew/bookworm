import MarkdownIt from 'markdown-it';

// Model output is untrusted: disable raw HTML and use MarkdownIt's safe URL validation.
const markdown = new MarkdownIt({ html: false, breaks: true, linkify: false });
export function renderMarkdown(text: string): string {
  return markdown.render(text);
}
