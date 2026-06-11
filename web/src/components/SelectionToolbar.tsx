// Floating "Comment" button that appears above the current selection.
export function SelectionToolbar({
  x,
  y,
  onComment,
}: {
  x: number;
  y: number;
  onComment: () => void;
}) {
  return (
    <div
      className="sel-toolbar"
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
