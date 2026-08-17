# UI layout test rules

These rules apply to all browser UI and layout tests. They are mandatory, not
preferences.

## Locators

- Query by role + accessible name. Fall back to `data-testid`.
- NEVER locate by CSS class, DOM index, XPath, or coordinates.
- If an element has no accessible name, fix the component, don't work around it
  in the test.

## Waiting

- NEVER use `page.waitForTimeout` or any sleep.
- Wait on state: `expect(locator).toBeVisible()`, network idle for a specific
  request, or a testid that appears when render settles.
- Disable animations globally in the fixture (`prefers-reduced-motion`,
  `* { animation: none !important }`).

## Assertions

- An assertion may only be weakened with a written justification in the test
  file explaining why the stricter form is wrong.
- If a test fails, the default assumption is that the APP is broken. Do not
  adjust thresholds, add retries, or narrow selectors to make it pass. Report
  the failure and stop.
- Every new assertion helper must be proven to fail: inject a synthetic defect,
  show the red run, remove it, show the green run. Include both in your report.

## Layout invariants (non-negotiable)

For every interactive element, after scrolling it into view:

1. Its bounding box is fully inside the visual viewport.
2. `document.elementFromPoint` at its center returns it or a descendant.
3. Its hit box is >= 44x44 CSS px (Apple HIG minimum).
4. It is reachable by sequential focus traversal from the document start.
5. No ancestor clips it: `scrollWidth <= clientWidth` unless the element is an
   intentional scroll container (marked `data-scroll="true"`).

## Prohibited

- No `--ignore-https-errors`, no disabling web security.
- No skipping/quarantining a test without a linked issue in the file.
- No screenshot baselines committed without a device+OS label in the name.
