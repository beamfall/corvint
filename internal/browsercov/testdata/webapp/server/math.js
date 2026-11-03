function add(a, b) {
  return a + b;
}

function sign(x) {
  if (x < 0) {
    return "negative";
  }
  return "positive";
}

module.exports = { add, sign };
