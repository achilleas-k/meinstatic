# TODO

## Dates on each post

Currently:
Can add dates anywhere on the page simply as text.  No biggie.

Cool idea:
Automatically add date from git.  Can show both "first created" and "last edited" dates and times.
I can take this even further and show a "created" date at the top (file first appeared in git),  "published" at the bottom (maybe manually), and "last edited" below that (last file modification date)

## Multiple templates

Different templates for different pages.  The template can be different for non-post pages, like contact and about, than it is for the post pages.

One quick idea would be to have a template for each directory.  Directories without templates inherit the parent directory's template.

Another idea would be to be able to specify the template name in each `.md` file, which the renderer would remove from the text during rendering.  This might lead to having a header with a whole set of metadata for each page, especially for posts.

## Relative paths

I could define variables that can be used in templates and get populated at render time.  For instance a variable can be used in the top menu to specify whether the links should be relative to the web root and add `../` accordingly.  The renderer should detect the depth of the `.md` source file in the source tree.

## Old items (to review)

- Skip hidden files and dirs in copyResources()
