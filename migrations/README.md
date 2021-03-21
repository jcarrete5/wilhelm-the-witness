Migrations
==========
This directory contains SQL migrations. Each file is named like "####-name.sql"
and will be read in lexicographic order by the migrate script. The preamble
file will be prepended to every migration before being interpreted by sqlite.
