export function History() {
  return <main className="mx-auto w-full max-w-6xl flex-1 px-6 py-8"><h1 className="text-2xl font-semibold">Geçmiş raporlar</h1></main>
}
export function ReportPage({ id }: { id: string }) {
  return <main className="mx-auto w-full max-w-6xl flex-1 px-6 py-8"><h1 className="font-mono">{id}</h1></main>
}
