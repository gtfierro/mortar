use warp::Filter;
use std::env;
use std::str;
use serde::{Serialize, Deserialize};
use std::collections::HashMap;
use tokio::time;
use futures::channel::mpsc;
use futures::future::join_all;
use futures::{
    stream, FutureExt, StreamExt, TryStreamExt,
};
use rdf::{node::Node, uri::Uri};
use tokio_postgres::{NoTls, Error, AsyncMessage};
use oxigraph::sparql::{QueryOptions, QueryResults, QueryResultsFormat};
use oxigraph::model::*;
use oxigraph::SledStore;
use reasonable::graphmanager::GraphManager;
use bb8::Pool;
use bb8_postgres::PostgresConnectionManager;

#[allow(non_upper_case_globals)]
const qfmt: &str = "PREFIX brick: <https://brickschema.org/schema/Brick#>
    PREFIX tag: <https://brickschema.org/schema/BrickTag#>
    PREFIX rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#>
    PREFIX rdfs: <http://www.w3.org/2000/01/rdf-schema#>
    PREFIX owl: <http://www.w3.org/2002/07/owl#>
    PREFIX qudt: <http://qudt.org/schema/qudt/>
    ";

fn with_db(store: SledStore) -> impl Filter<Extract = (SledStore,), Error = std::convert::Infallible> + Clone {
    warp::any().map(move || store.clone())
}

#[derive(Serialize, Deserialize, Debug)]
struct EmbeddedTriple {
    source: String,
    s: String,
    p: String,
    o: String,
}
#[derive(Serialize, Deserialize, Debug)]
struct TripleEvent {
    table: String,
    action: String,
    data: EmbeddedTriple
}

#[derive(Debug)]
struct NotUtf8;
impl warp::reject::Reject for NotUtf8 {}

fn parse_triple_term(t: &str) -> Option<Node> {
    let r = match t.as_bytes()[0] as char {
        '<' => Node::UriNode { uri: Uri::new(t.replace(&['<','>'][..], "")) },
        '_' => Node::BlankNode { id: t.to_string() },
        '"' => Node::LiteralNode { literal: t.to_string(), data_type: None, language: None },
        _ => return None
    };
    Some(r)
}

fn graphname_node(t: &str) -> NamedOrBlankNode {
    NamedOrBlankNode::NamedNode(NamedNode::new(format!("urn:{}", t)).unwrap())
}

#[tokio::main]
async fn main() -> Result<(), Error> {
    let mut mgr = GraphManager::new();
    // mgr.load_file("/home/gabe/src/Brick/Brick/Brick.ttl").unwrap();
    // mgr.load_file("/home/gabe/src/Brick/Brick/examples/soda_brick.ttl").unwrap();
    //mgr.load_file("~/src/Brick/Brick/examples/soda_brick.ttl").unwrap();
    // let store = mgr.store();
    let store = mgr.store();
    println!("Loaded files");

    // Connect to the database.
    let connstr = format!("host={} port={} dbname={} user={} password={}",
         env::var("MORTAR_DB_HOST").unwrap_or("localhost".to_string()),
         env::var("MORTAR_DB_PORT").unwrap_or("5432".to_string()),
         env::var("MORTAR_DB_DATABASE").unwrap_or("mortar".to_string()),
         env::var("MORTAR_DB_USER").unwrap_or("mortarchangeme".to_string()),
         env::var("MORTAR_DB_PASSWORD").unwrap_or("mortarpasswordchangeme".to_string())
    );

    let pg_mgr = PostgresConnectionManager::new_from_stringlike(&connstr, NoTls).unwrap();
    let pool = match Pool::builder().build(pg_mgr).await {
        Ok(pool) => pool,
        Err(e) => panic!("bb8 error {}", e),
    };

    let (client, mut connection) =
        tokio_postgres::connect(&connstr, NoTls).await?;

    let (tx, mut rx) = mpsc::unbounded();
    let stream = stream::poll_fn(move |cx| connection.poll_message(cx)).map_err(|e| panic!(e));
    let connection = stream.forward(tx).map(|r| r.unwrap());

    // The connection object performs the actual communication with the database,
    // so spawn it off to run on its own.
    tokio::spawn(async move {
        connection.await;
        // if let Err(e) = connection.await {
        //     eprintln!("connection error: {}", e);
        // }
    });

    // get a vec of all sources
    let rows = client.query("SELECT distinct source FROM latest_triples", &[]).await?;
    let sources = rows.iter().map(|row| {
        let src: &str = row.get(0);
        src.to_owned()
    });

    let ingest_futures = sources.map(|src| {
        let newp = pool.clone();
        async move {
            let sourcename = src.clone();
            let conn = newp.get().await.unwrap();
            let rows = conn.query("SELECT s, p, o FROM latest_triples WHERE source = $1", &[&src]).await.unwrap();
            let triples: Vec<(Node, Node, Node)> = rows.iter().map(|row| {
                let (s, p, o): (&str, &str, &str) = (row.get(0), row.get(1), row.get(2));
                (parse_triple_term(s).unwrap(), parse_triple_term(p).unwrap(), parse_triple_term(o).unwrap())
            }).collect();
            (sourcename, triples)
        }
    });
    let _: Vec<_> = join_all(ingest_futures).await.into_iter().map(|(sourcename, triples)| {
        mgr.add_triples(Some(sourcename), triples);
    }).collect();


    // TODO: use source name in latest triples to do this for each graph
    //let rows = client.query("SELECT s, p, o, source FROM latest_triples", &[]).await?;
    //let mut values: HashMap<String, Vec<(Node, Node, Node)>> = HashMap::new();
    //for row in rows {
    //    let (s, p, o): (&str, &str, &str) = (row.get(0), row.get(1), row.get(2));
    //    let source: &str = row.get(3);
    //    values.entry(source.to_owned())
    //          .or_insert(Vec::new())
    //          .push((parse_triple_term(s).unwrap(), parse_triple_term(p).unwrap(), parse_triple_term(o).unwrap()));
    //}
    //for (graphname, v) in values.iter() {
    //    println!("graph {} triples: {}", graphname, v.len());
    //    mgr.add_triples(Some(graphname.clone()), v.clone());
    //}
    //let v: Vec<(String, Node, Node, Node)> = rows.iter().filter_map(|row| {
    //}).collect();


    // subscribe
    client.execute("LISTEN events;", &[]).await?;

    // TODO: graph name in the query
    let query = warp::path!("query" / String)
            .and(warp::body::content_length_limit(1024))
            .and(
                warp::body::bytes().and_then(|body: bytes::Bytes| async move {
                    std::str::from_utf8(&body)
                        .map(String::from)
                        .map_err(|_e| warp::reject::custom(NotUtf8))
                }),
            )
            .and(with_db(store.clone()))
            .map(|graphname: String, query: String, store: SledStore| {
                let sparql = format!("{}{}", qfmt, query);
                //println!("query: {} {}", sparql, graphname);
                let opts = match graphname.as_str() {
                    "default" => QueryOptions::default().with_default_graph_as_union(),
                    "all" => QueryOptions::default().with_default_graph_as_union(),
                    gname => QueryOptions::default().with_default_graph(graphname_node(gname)) ,
                };
                println!("Querying graph {}", graphname_node(&graphname));
                let q = store.clone().prepare_query(&sparql, opts).unwrap();
                let res = q.exec().unwrap();
                let mut resp: Vec<u8> = Vec::new();
                if let QueryResults::Solutions(_) = res {
                    res.write(&mut resp, QueryResultsFormat::Json).unwrap();
                    warp::http::Response::builder()
                        .header("content-type", "application/json")
                        .body(resp)
                } else {
                    warp::http::Response::builder()
                        .status(warp::http::StatusCode::INTERNAL_SERVER_ERROR)
                        .body("No results".as_bytes().to_vec())
                }
            });

    let query2 = warp::path!("query" / String)
            .and(warp::body::content_length_limit(1024))
            .and(warp::header::exact("content-type", "application/x-www-form-urlencoded"))
            .and(warp::body::form())
            .and(with_db(store.clone()))
            .map(|graphname: String, m: HashMap<String, String>, store: SledStore| {
                if let Some(query) = m.get("query") {
                    let sparql = format!("{}{}", qfmt, query);
                    //println!("query: {} {}", sparql, graphname);
                    let opts = match graphname.as_str() {
                        "default" => QueryOptions::default().with_default_graph_as_union(),
                        "all" => QueryOptions::default().with_default_graph_as_union(),
                        gname => QueryOptions::default().with_default_graph(graphname_node(gname)) ,
                    };
                    println!("Querying graph {}", graphname_node(&graphname));
                    let q = store.clone().prepare_query(&sparql, opts).unwrap();
                    let res = q.exec().unwrap();
                    let mut resp: Vec<u8> = Vec::new();
                    if let QueryResults::Solutions(_) = res {
                        res.write(&mut resp, QueryResultsFormat::Json).unwrap();
                        warp::http::Response::builder()
                            .header("content-type", "application/json")
                            .body(resp)
                    } else {
                        warp::http::Response::builder()
                            .status(warp::http::StatusCode::INTERNAL_SERVER_ERROR)
                            .body("No results".as_bytes().to_vec())

                    }
                } else {
                    warp::http::Response::builder()
                        .status(warp::http::StatusCode::INTERNAL_SERVER_ERROR)
                        .body("Bad query".as_bytes().to_vec())

                }
            });

    // TODO: accept postgres stuff as env variables
    println!("Serving on 0.0.0.0:3030");
    tokio::spawn(
        warp::serve(query2.or(query))
            .run(([0, 0, 0, 0], 3030))
    );

    //let mut trips: Vec<(Node, Node, Node)> = Vec::new();
    let mut trips: HashMap<String, Vec<(Node, Node, Node)>> = HashMap::new();
    let mut interval = time::interval(time::Duration::from_secs(10));
    loop {
        tokio::select! {
            msg = rx.next() => {
                if let Some(x) = msg {
                    if let AsyncMessage::Notification(n) = x {
                        let msg: TripleEvent = serde_json::from_str(n.payload()).unwrap();
                        let source = msg.data.source.to_owned();
                        trips.entry(source).or_insert(Vec::new())
                             .push((parse_triple_term(&msg.data.s).unwrap(),
                                    parse_triple_term(&msg.data.p).unwrap(),
                                    parse_triple_term(&msg.data.o).unwrap()));
                    }
                }
            },
            _ = interval.tick() => {
                if trips.len() > 0 {
                    for (graphname, values) in trips.iter() {
                        mgr.add_triples(Some(graphname.clone()), values.clone());
                        println!("Integrated {} {}", graphname, values.len());
                    }
                    trips.clear();
                }
            }
        }
    }
}
