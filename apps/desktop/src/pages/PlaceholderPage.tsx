type PlaceholderPageProps = {
  title: string;
  items: string[];
};

export function PlaceholderPage({
  title,
  items,
}: PlaceholderPageProps): JSX.Element {
  return (
    <div className="page">
      <section className="placeholder-panel">
        <h2>{title}</h2>
        <div className="placeholder-list">
          {items.map((item) => (
            <span key={item}>{item}</span>
          ))}
        </div>
      </section>
    </div>
  );
}
