import { useParams } from "react-router-dom";
import { Markdown } from "@yetibyrash/ui";
import { api } from "../api";
import { ErrorMessage, Loading } from "../components/Status";
import { useApi } from "../useApi";

// About, FAQ, policies: pages written on the admin's Homepage and content screen.
export function InfoPage() {
  const { slug = "" } = useParams();
  const { data, error } = useApi(`page:${slug}`, (s) => api.page(slug, s));
  if (error) return <ErrorMessage error={error} />;
  if (!data) return <Loading />;
  return (
    <article className="stack info-page">
      <h1 className="page-title">{data.title}</h1>
      <Markdown source={data.body} className="prose" />
    </article>
  );
}
