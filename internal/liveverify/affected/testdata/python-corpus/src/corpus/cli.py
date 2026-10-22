from corpus import query, render


def main(text):
    return render.table([query.run(text)])
