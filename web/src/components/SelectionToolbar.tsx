// Comment affordance shown while text is selected.
//
// Two presentations, chosen by CSS (see `pointer: coarse` rules in styles.css):
//   - Mouse/desktop: a floating pill anchored above (or below) the selection.
//   - Touch/phones:  a bar docked to the top of the viewport. iOS draws its own
//     native callout (Copy / Look Up / Translate…) right over the selection,
//     covering a floating pill — a docked bar stays clear of it.
// Both act on the same already-captured selection anchor.
import type { Strings } from "../i18n";

export function SelectionToolbar({
  x,
  y,
  flip,
  onComment,
  t,
}: {
  x: number;
  y: number;
  flip: boolean;
  onComment: () => void;
  t: Strings;
}) {
  // Use mousedown/touchstart so the selection isn't cleared before we read it.
  const press = (e: React.SyntheticEvent) => {
    e.preventDefault();
    onComment();
  };
  return (
    <>
      <div
        className={`sel-toolbar${flip ? " sel-toolbar--below" : ""}`}
        style={{ left: x, top: y }}
        onMouseDown={press}
        onTouchStart={press}
      >
        {t.comment}
      </div>
      <div className="sel-bar" onMouseDown={press} onTouchStart={press}>
        <span className="sel-bar-hint">{t.textSelected}</span>
        <span className="sel-bar-btn">{t.comment}</span>
      </div>
    </>
  );
}
