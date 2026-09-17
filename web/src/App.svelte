<script>
  import { onMount } from 'svelte'

  let query = ''
  let sections = []
  let loading = true
  let error = ''
  let formOpen = false
  let title = ''
  let summary = ''
  let saving = false

  async function loadSections() {
    loading = true
    error = ''
    try {
      const response = await fetch(`/api/sections?q=${encodeURIComponent(query)}`)
      if (!response.ok) throw new Error('Не вдалося отримати дані.')
      sections = await response.json()
    } catch (cause) {
      error = cause.message
    } finally {
      loading = false
    }
  }

  async function saveSection() {
    saving = true
    error = ''
    try {
      const response = await fetch('/api/sections', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title, summary })
      })
      const payload = await response.json()
      if (!response.ok) throw new Error(payload.error || 'Не вдалося зберегти.')
      title = ''
      summary = ''
      formOpen = false
      await loadSections()
    } catch (cause) {
      error = cause.message
    } finally {
      saving = false
    }
  }

  onMount(loadSections)
</script>

<svelte:head><meta name="description" content="Мінімальний help portal для Koyeb" /></svelte:head>

<main>
  <section class="hero">
    <p class="eyebrow">Go + Chi · Svelte · Koyeb</p>
    <h1>Портал інструкцій<br /><em>працює.</em></h1>
    <p class="lede">Мінімальний стенд для перевірки Docker-деплою, API та Svelte-інтерфейсу.</p>
    <div class="status"><span></span> Сервіс готовий відповідати</div>
  </section>

  <section class="workspace" aria-label="Інструкції">
    <div class="toolbar">
      <label class="search">
        <span>Пошук</span>
        <input bind:value={query} oninput={loadSections} placeholder="Наприклад, погодження" />
      </label>
      <button class="add" onclick={() => formOpen = !formOpen}>{formOpen ? 'Закрити форму' : '+ Тестовий розділ'}</button>
    </div>

    {#if formOpen}
      <form class="test-form" onsubmit={(event) => { event.preventDefault(); saveSection() }}>
        <label>Назва <input required bind:value={title} maxlength="100" /></label>
        <label>Короткий опис <input required bind:value={summary} maxlength="240" /></label>
        <button disabled={saving}>{saving ? 'Збереження…' : 'Додати в пам’ять'}</button>
        <small>Це демонстраційний API: дані зникнуть після перезапуску контейнера.</small>
      </form>
    {/if}

    {#if error}<p class="error" role="alert">{error}</p>{/if}
    {#if loading}
      <p class="empty">Завантажуємо інструкції…</p>
    {:else if sections.length === 0}
      <p class="empty">За цим запитом інструкцій не знайдено.</p>
    {:else}
      <div class="cards">
        {#each sections as section, index}
          <article>
            <p class="number">0{index + 1}</p>
            <h2>{section.title}</h2>
            <p>{section.summary}</p>
            <a href={'#section-' + section.id}>Відкрити <span>→</span></a>
          </article>
        {/each}
      </div>
    {/if}
  </section>
</main>
