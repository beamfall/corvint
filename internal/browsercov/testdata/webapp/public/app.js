async function show(url) {
  const r = await fetch(url);
  document.getElementById("out").textContent = String((await r.json()).result);
}

function onAdd() {
  show("/api/add?a=2&b=3");
}

function onSign() {
  show("/api/sign?x=4");
}

document.getElementById("add").addEventListener("click", onAdd);
document.getElementById("sign").addEventListener("click", onSign);
