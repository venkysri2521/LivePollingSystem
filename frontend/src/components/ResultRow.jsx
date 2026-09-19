/**
 * One option line. The result fills in behind the label like a highlighter
 * stroke across a ballot, so the number and the mark are the same object
 * rather than a label with a bar underneath it.
 */
export default function ResultRow({ option, count, total, share, selectable, selected, onSelect, leading, flashing, revealed }) {
  const percent = total > 0 ? Math.round(share * 1000) / 10 : 0;

  const content = (
    <>
      <span
        className="row__fill"
        style={{ transform: `scaleX(${revealed ? share : 0})` }}
        aria-hidden="true"
      />
      <span className="row__label">{option.text}</span>
      {revealed && (
        <span className="row__figures">
          <span className="row__percent">{percent}%</span>
          <span className="row__count">
            {count} {count === 1 ? "vote" : "votes"}
          </span>
        </span>
      )}
    </>
  );

  const classes = [
    "row",
    leading && revealed && total > 0 ? "row--leading" : "",
    flashing ? "row--flash" : "",
    selected ? "row--selected" : "",
  ]
    .filter(Boolean)
    .join(" ");

  if (!selectable) {
    return <div className={classes}>{content}</div>;
  }

  return (
    <button type="button" className={classes} onClick={() => onSelect(option.id)} aria-pressed={selected}>
      {content}
    </button>
  );
}
