
import AppsCard from "@/components/AppsCard";
import UsersCard from "@/components/UsersCard";

export default function Home() {
  return (
    <main
      className="flex flex-wrap gap-2 p-4 items-start"
    >
      <AppsCard className="flex-2" />
      <UsersCard className="flex-1" />
    </main>
  );
}
