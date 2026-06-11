// Floating "Comment" button that appears above (or below) the current selection.
export function SelectionToolbar({
  x,
  y,
  flip,
  onComment,
}: {
  x: number;
  y: number;
  flip: boolean;
  onComment: () => void;
}) {
  return (
    <div
      className={`sel-toolbar${flip ? " sel-toolbar--below" : ""}`}
      style={{ left: x, top: y }}
      // Use mousedown/touchstart so the selection isn't cleared before we read it.
      onMouseDown={(e) => {
        e.preventDefault();
        onComment();
      }}
      onTouchStart={(e) => {
        e.preventDefault();
        onComment();
      }}
    >
      💬 Comment
    </div>
  );
}
